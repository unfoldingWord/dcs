// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"fmt"
	"sort"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/models/repo"

	"xorm.io/builder"
)

// SearchCatalog returns catalog repositories based on search options,
// it returns results in given range and number of total results.
func SearchCatalog(ctx context.Context, opts *door43metadata.SearchCatalogOptions) (repo.Door43MetadataList, int64, error) {
	cond := door43metadata.SearchCatalogCondition(opts)
	return SearchCatalogByCondition(ctx, opts, cond)
}

// SearchCatalogByCondition search repositories by condition
func SearchCatalogByCondition(ctx context.Context, opts *door43metadata.SearchCatalogOptions, cond builder.Cond) (repo.Door43MetadataList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize < 0 {
		opts.PageSize = 0
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = []door43metadata.CatalogOrderBy{door43metadata.CatalogOrderByNewest}
	}

	var dms repo.Door43MetadataList
	if opts.PageSize > 0 {
		dms = make(repo.Door43MetadataList, 0, opts.PageSize)
	}

	releaseInfoInner, err := builder.Select("`door43_metadata`.repo_id", "COUNT(*) AS release_count", "MAX(`door43_metadata`.release_date_unix) AS latest_unix").
		From("door43_metadata").
		GroupBy("`door43_metadata`.repo_id").
		Where(builder.Gt{"`door43_metadata`.release_date_unix": 0}).
		Where(door43metadata.GetStageCond(opts.Stage)).
		ToBoundSQL()
	if err != nil {
		return nil, 0, err
	}

	releaseInfoOuter, err := builder.Select("`door43_metadata`.repo_id", "MAX(release_count) AS release_count", "MAX(latest_unix) AS latest_unix", "MIN(stage) AS latest_stage").
		From("door43_metadata").
		Join("INNER", "("+releaseInfoInner+") release_info_inner", "`release_info_inner`.repo_id = `door43_metadata`.repo_id AND `door43_metadata`.release_date_unix = `release_info_inner`.latest_unix").
		GroupBy("`door43_metadata`.repo_id").
		ToBoundSQL()
	if err != nil {
		return nil,
			0, err
	}

	sess := db.GetEngine(db.DefaultContext).
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Join("INNER", "user", "`repository`.owner_id = `user`.id").
		Join("LEFT", "release", "`release`.id = `door43_metadata`.release_id").
		Join("INNER", "("+releaseInfoOuter+") release_info", "release_info.repo_id = `door43_metadata`.repo_id").
		Where(cond)

	for _, orderBy := range opts.OrderBy {
		sess.OrderBy(orderBy.String())
	}

	if opts.PageSize > 0 || opts.Page > 1 {
		sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	count, err := sess.FindAndCount(&dms)
	if err != nil {
		return nil, 0, fmt.Errorf("FindAndCount: %v", err)
	}

	if err = dms.LoadAttributes(ctx); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
	}

	return dms, count, nil
}

// SearchCatalogLanguages returns a list of unique strings of languages in the catalog for the given criteria
func SearchCatalogLanguages(ctx context.Context, opts *door43metadata.SearchCatalogOptions) ([]string, error) {
	dms, _, err := SearchCatalog(ctx, opts)
	if err != nil {
		return nil, err
	}

	fieldMap := map[string]bool{}
	for _, dm := range dms {
		fieldMap[dm.Language] = true
	}
	keys := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// SearchCatalogSubjects returns a list of unique strings of subjects in the catalog for the given criteria
func SearchCatalogSubjects(ctx context.Context, opts *door43metadata.SearchCatalogOptions) ([]string, error) {
	dms, _, err := SearchCatalog(ctx, opts)
	if err != nil {
		return nil, err
	}

	fieldMap := map[string]bool{}
	for _, dm := range dms {
		fieldMap[dm.Subject] = true
	}
	keys := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// SearchCatalogOwners returns a list of unique strings of owner names in the catalog for the given criteria
func SearchCatalogOwners(ctx context.Context, opts *door43metadata.SearchCatalogOptions) ([]string, error) {
	dms, _, err := SearchCatalog(ctx, opts)
	if err != nil {
		return nil, err
	}

	fieldMap := map[string]bool{}
	for _, dm := range dms {
		if err := dm.LoadRepo(ctx); err != nil || dm.Repo == nil {
			continue
		}
		if err := dm.Repo.LoadOwner(ctx); err != nil || dm.Repo.Owner == nil {
			continue
		}
		fieldMap[dm.Repo.Owner.Name] = true
	}
	keys := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// SearchCatalogMetadataTypes returns a list of unique strings of metadata tyupes in the catalog for the given criteria
func SearchCatalogMetadataTypes(ctx context.Context, opts *door43metadata.SearchCatalogOptions) ([]string, error) {
	dms, _, err := SearchCatalog(ctx, opts)
	if err != nil {
		return nil, err
	}

	fieldMap := map[string]bool{}
	for _, dm := range dms {
		fieldMap[dm.MetadataType] = true
	}
	keys := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}
