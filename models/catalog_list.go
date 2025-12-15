// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"fmt"
	"strings"

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
		return nil, 0, err
	}

	releaseInfoJoinCondition := "release_info.repo_id = `door43_metadata`.repo_id"
	if !opts.IncludeHistory {
		releaseInfoJoinCondition += " AND release_info.latest_unix = `door43_metadata`.release_date_unix AND release_info.latest_stage = `door43_metadata`.stage"
	}

	sess := db.GetEngine(ctx).
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Join("INNER", "user", "`repository`.owner_id = `user`.id").
		Join("LEFT", "release", "`release`.id = `door43_metadata`.release_id").
		Join("INNER", "("+releaseInfoOuter+") release_info", releaseInfoJoinCondition).
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

// SearchDoor43MetadataField returns door43metadat field based on search options
func SearchDoor43MetadataField(ctx context.Context, opts *door43metadata.SearchCatalogOptions, field string) ([]string, error) {
	cond := door43metadata.SearchCatalogCondition(opts)
	return SearchDoor43MetadataFieldByCondition(ctx, opts, cond, field)
}

// SearchDoor43MetadataFieldByCondition search door43metadata entries by condition for a single field
func SearchDoor43MetadataFieldByCondition(ctx context.Context, opts *door43metadata.SearchCatalogOptions, cond builder.Cond, field string) ([]string, error) {
	var results []string

	if !strings.Contains(field, ".") {
		field = "`door43_metadata`." + field
	}

	sess := db.GetEngine(ctx).Table("door43_metadata").
		Distinct(field).
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Join("INNER", "user", "`repository`.owner_id = `user`.id").
		Where(cond).
		And(builder.Neq{field: ""}).
		OrderBy(field)

	err := sess.Find(&results)
	if err != nil {
		return nil, fmt.Errorf("find: %v", err)
	}

	return results, nil
}

// SearchCatalogForBookPackage returns catalog repositories based on search options for a book package,
// it returns results in given range and number of total results.
func SearchCatalogForBookPackage(ctx context.Context, ref string, opts *door43metadata.SearchCatalogOptions) (repo.Door43MetadataList, int64, error) {
	books := opts.Books
	opts.Books = nil
	bookCond := builder.NewCond()

	if len(books) > 0 {
		innerBookCond := builder.NewCond()
		for _, book := range books {
			for _, v := range strings.Split(book, ",") {
				innerBookCond = innerBookCond.And(builder.Expr("JSON_SEARCH(dm.ingredients, 'one', ? COLLATE utf8mb4_general_ci, NULL, '$[*].identifier') IS NOT NULL", strings.ToLower(v)))
			}
		}
		bookCond = builder.Or(
			builder.In("`door43_metadata`.subject", []string{"Translation Academy", "Translation Words"}),
			innerBookCond,
		)
	}

	cond := door43metadata.SearchCatalogCondition(opts)
	cond = cond.And(bookCond)
	return SearchCatalogForBookPackageByCondition(ctx, ref, opts, cond)
}

// SearchCatalogForBookPackageByCondition search repositories by condition for a book package
func SearchCatalogForBookPackageByCondition(ctx context.Context, ref string, opts *door43metadata.SearchCatalogOptions, cond builder.Cond) (repo.Door43MetadataList, int64, error) {
	// Extract owner and repo from opts (should have exactly one each)
	if len(opts.Owners) == 0 || len(opts.Repos) == 0 {
		return nil, 0, fmt.Errorf("owner and repo must be specified in search options")
	}
	owner := opts.Owners[0]
	repoName := opts.Repos[0]

	// Build the WHERE clause from the builder.Cond for use in the filtered CTE
	// We need to convert the builder conditions to SQL that can be used in the native query
	condSQL, condArgs, err := builder.ToSQL(cond)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build condition SQL: %v", err)
	}

	// Replace table aliases in the condition to match the CTE context
	// In the filtered CTE, we use 'dm' as the alias for door43_metadata table
	condSQL = strings.ReplaceAll(condSQL, "`door43_metadata`.", "dm.")
	condSQL = strings.ReplaceAll(condSQL, "`repository`.", "r.")
	condSQL = strings.ReplaceAll(condSQL, "`user`.", "u.")

	// Build the complete query with CTEs
	query := `
WITH anchor AS (
  SELECT
    dm.*,
    r.owner_id
  FROM door43_metadata dm
  JOIN repository r ON r.id = dm.repo_id
  JOIN user u       ON u.id = r.owner_id
  WHERE
    u.lower_name = ?
    AND r.name = ?
    AND dm.ref = ?
  LIMIT 1
),
filtered AS (
  SELECT
    FROM_UNIXTIME(dm.release_date_unix) AS released,
    dm.*,
    ABS(dm.release_date_unix - a.release_date_unix) AS time_diff
  FROM door43_metadata dm
  JOIN repository r ON r.id = dm.repo_id
  JOIN user u       ON u.id = r.owner_id
  JOIN anchor a
  WHERE
    dm.stage <= a.stage
    AND dm.language = a.language
    AND r.owner_id = a.owner_id
    AND (` + condSQL + `)
),
ranked AS (
  SELECT
    f.*,
    ROW_NUMBER() OVER (
      PARTITION BY f.abbreviation
      ORDER BY f.time_diff ASC, f.release_date_unix ASC
    ) AS rn
  FROM filtered f
)
SELECT 
  id, repo_id, release_id, ref, ref_type, commit_sha, stage,
  metadata_type, metadata_version, subject, flavor_type, flavor,
  abbreviation, title, publisher, language, language_title,
  language_direction, language_is_gl, content_format, checking_level,
  ingredients, relations, is_latest_for_stage, is_repo_metadata,
  metadata, validation_error, healthcheck_severity, healthcheck_counts,
  release_date_unix, created_unix, updated_unix
FROM ranked
WHERE rn = 1
ORDER BY abbreviation`

	// Prepare query arguments: owner, repoName, ref, then all condition args
	queryArgs := []interface{}{owner, repoName, ref}
	queryArgs = append(queryArgs, condArgs...)

	// Execute the query
	var dms repo.Door43MetadataList
	err = db.GetEngine(ctx).SQL(query, queryArgs...).Find(&dms)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %v", err)
	}

	// Load attributes for the results
	if err = dms.LoadAttributes(ctx); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
	}

	return dms, int64(len(dms)), nil
}
