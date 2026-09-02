// Copyright 2026 unfoldingWord. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"
	"sync/atomic"

	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

// fullTextIndexName is the FULLTEXT index GetMetadataCond's MySQL path relies on. It
// covers title, subject and language_title with the ngram parser, which indexes
// overlapping character n-grams rather than splitting on word boundaries only, so
// MATCH...AGAINST can approximate the substring matching the LIKE queries it replaces
// were doing (leading-wildcard LIKE, which MySQL can't use any index for).
const fullTextIndexName = "dcs_ft_search"

// languageLowerIndexName is a functional index on LOWER(language): the language match
// in GetMetadataCond is case-insensitive, and a plain index on language can't be used
// once the column reference is wrapped in LOWER().
const languageLowerIndexName = "dcs_idx_language_lower"

// fullTextIndexReady reports whether EnsureFullTextIndexes has confirmed both indexes
// exist. GetMetadataCond checks this before using MATCH...AGAINST, so a not-yet-run or
// failed index creation degrades search to LIKE instead of a MySQL query error.
var fullTextIndexReady atomic.Bool

// EnsureFullTextIndexes creates the door43_metadata indexes GetMetadataCond's MySQL
// path needs, if they don't already exist. It is a no-op outside MySQL.
//
// This can't be a one-time migration: xorm's Sync (which runs on every server boot)
// drops any index it finds that isn't declared on a Go struct field, and xorm's index
// model has no FULLTEXT or functional-index concept — a migration-created index here
// would be dropped again on the very next restart. Calling this after Sync has already
// run, on every boot, is what makes it survive; it costs one metadata lookup when the
// indexes already exist.
//
// Errors are logged, not returned, so a transient failure here degrades search rather
// than fails the whole server boot (this is called via mustInitCtx in routers/init.go,
// which treats a returned error as fatal).
func EnsureFullTextIndexes(ctx context.Context) {
	if !setting.Database.Type.IsMySQL() {
		return
	}

	existing, err := existingIndexNames(ctx, "door43_metadata")
	if err != nil {
		log.Error("door43metadata.EnsureFullTextIndexes: checking existing indexes: %v", err)
		return
	}

	if !existing[fullTextIndexName] {
		if _, err := db.GetEngine(ctx).Exec("ALTER TABLE `door43_metadata` ADD FULLTEXT INDEX `" + fullTextIndexName + "` (`title`, `subject`, `language_title`) WITH PARSER ngram"); err != nil {
			log.Error("door43metadata.EnsureFullTextIndexes: creating FULLTEXT index: %v", err)
			return
		}
	}

	if !existing[languageLowerIndexName] {
		if _, err := db.GetEngine(ctx).Exec("ALTER TABLE `door43_metadata` ADD INDEX `" + languageLowerIndexName + "` ((LOWER(`language`)))"); err != nil {
			log.Error("door43metadata.EnsureFullTextIndexes: creating language index: %v", err)
			return
		}
	}

	fullTextIndexReady.Store(true)
}

func existingIndexNames(ctx context.Context, table string) (map[string]bool, error) {
	var names []string
	if err := db.GetEngine(ctx).SQL(
		"SELECT DISTINCT INDEX_NAME FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", table,
	).Find(&names); err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set, nil
}

// fullTextSearchAvailable reports whether GetMetadataCond can safely use
// MATCH...AGAINST against the FULLTEXT index EnsureFullTextIndexes creates.
func fullTextSearchAvailable() bool {
	return setting.Database.Type.IsMySQL() && fullTextIndexReady.Load()
}
