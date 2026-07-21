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
	"code.gitea.io/gitea/modules/structs"

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

	// Convert builder condition to parameterized SQL
	condSQL, condArgs, err := builder.ToSQL(cond)
	if err != nil {
		return nil, 0, err
	}

	// Translate table references to match CTE aliases (dm, r, u, rel)
	condSQL = strings.ReplaceAll(condSQL, "`door43_metadata`.", "dm.")
	condSQL = strings.ReplaceAll(condSQL, "`repository`.", "r.")
	condSQL = strings.ReplaceAll(condSQL, "`user`.", "u.")
	condSQL = strings.ReplaceAll(condSQL, "`release`.", "rel.")

	// Translate ORDER BY column references for the outer query (no table prefix needed)
	orderParts := make([]string, 0, len(opts.OrderBy))
	for _, ob := range opts.OrderBy {
		s := strings.ReplaceAll(ob.String(), "`door43_metadata`.", "")
		s = strings.ReplaceAll(s, "`repository`.", "")
		orderParts = append(orderParts, s)
	}
	orderSQL := strings.Join(orderParts, ", ")

	// When not including history, use ROW_NUMBER to pick the best (latest, lowest stage) row per repo
	rnSelect := ""
	rnFilter := ""
	if !opts.IncludeHistory {
		rnSelect = ",\n         ROW_NUMBER() OVER (PARTITION BY dm.repo_id ORDER BY dm.release_date_unix DESC, dm.stage ASC) AS _rn"
		rnFilter = "WHERE _rn = 1\n"
	}

	// The fat JSON columns (metadata, validation_error) are deliberately NOT
	// selected: the search condition already restricts results to rows whose
	// validation_error is NULL, and neither the catalog API nor the catalog
	// web page renders the raw metadata of search results. Materializing the
	// metadata MEDIUMTEXT for every row dominated the query cost.
	dmCols := []string{
		"id", "repo_id", "release_id", "ref", "ref_type", "commit_sha", "stage",
		"metadata_type", "metadata_version", "subject", "flavor_type", "flavor",
		"abbreviation", "title", "publisher", "language", "language_title",
		"language_direction", "language_is_gl", "content_format", "checking_level",
		"ingredients", "relations", "has_audio", "has_video", "has_pdf", "has_stream", "has_other",
		"is_latest_for_stage", "is_repo_metadata", "healthcheck_severity", "healthcheck_counts",
		"release_date_unix", "created_unix", "updated_unix",
	}
	searchCols := strings.Join(dmCols, ", ")
	innerCols := "dm." + strings.Join(dmCols, ", dm.")

	// CTE filters first (using indexes), then ranks — avoids full-table derived table materialization
	// Note: "release" is a MySQL reserved word and must be backtick-quoted in raw SQL.
	cteSQL := "WITH catalog_filtered AS (\n" +
		"  SELECT " + innerCols + ",\n" +
		"         r.lower_name AS lower_name, r.num_stars, r.num_forks,\n" +
		"         COUNT(*) OVER (PARTITION BY dm.repo_id) AS release_count" + rnSelect + "\n" +
		"  FROM door43_metadata dm\n" +
		"  INNER JOIN repository r ON r.id = dm.repo_id\n" +
		"  INNER JOIN user u ON r.owner_id = u.id\n" +
		"  LEFT JOIN `release` rel ON rel.id = dm.release_id\n" +
		"  WHERE " + condSQL + "\n)\n"

	// An unpaginated request returns every row, so the total is just the
	// result length — skip the separate COUNT pass over the same CTE.
	needsCount := opts.PageSize > 0 || opts.Page > 1
	var count int64
	if needsCount {
		if _, err = db.GetEngine(ctx).SQL(cteSQL+`SELECT COUNT(*) FROM catalog_filtered `+rnFilter, condArgs...).Get(&count); err != nil {
			return nil, 0, fmt.Errorf("count: %v", err)
		}
	}

	var dms repo.Door43MetadataList
	if !needsCount || count > 0 {
		if opts.PageSize > 0 {
			dms = make(repo.Door43MetadataList, 0, opts.PageSize)
		}

		limitSQL := ""
		if opts.PageSize > 0 || opts.Page > 1 {
			limitSQL = fmt.Sprintf("\nLIMIT %d OFFSET %d", opts.PageSize, (opts.Page-1)*opts.PageSize)
		}

		dataSQL := cteSQL + "SELECT " + searchCols + `,
  lower_name, num_stars, num_forks, release_count
FROM catalog_filtered
` + rnFilter + `ORDER BY ` + orderSQL + limitSQL

		if err = db.GetEngine(ctx).SQL(dataSQL, condArgs...).Find(&dms); err != nil {
			return nil, 0, fmt.Errorf("find: %v", err)
		}

		if err = dms.LoadAttributes(ctx); err != nil {
			return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
		}
	}
	if !needsCount {
		count = int64(len(dms))
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

	// Only join user when the selected field or conditions actually reference it — avoids unnecessary join overhead
	condSQL, _, _ := builder.ToSQL(cond)
	needsUserJoin := strings.Contains(field, "`user`.") || strings.Contains(condSQL, "`user`.")

	sess := db.GetEngine(ctx).Table("door43_metadata").
		Distinct(field).
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id")
	if needsUserJoin {
		sess.Join("INNER", "user", "`repository`.owner_id = `user`.id")
	}
	sess.Where(cond).
		And(builder.Neq{field: ""}).
		OrderBy(field)

	err := sess.Find(&results)
	if err != nil {
		return nil, fmt.Errorf("find: %v", err)
	}

	return results, nil
}

// SearchDoor43MetadataFieldCountsByCondition returns the entry count of each distinct
// non-empty value of field among the door43_metadata entries matching cond, keyed by value.
func SearchDoor43MetadataFieldCountsByCondition(ctx context.Context, cond builder.Cond, field string) (map[string]int64, error) {
	if !strings.Contains(field, ".") {
		field = "`door43_metadata`." + field
	}

	// Only join user when the selected field or conditions actually reference it — avoids unnecessary join overhead
	condSQL, _, _ := builder.ToSQL(cond)
	needsUserJoin := strings.Contains(field, "`user`.") || strings.Contains(condSQL, "`user`.")

	var rows []struct {
		FieldValue string
		Cnt        int64
	}
	sess := db.GetEngine(ctx).Table("door43_metadata").
		Select(field+" AS field_value, COUNT(*) AS cnt").
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id")
	if needsUserJoin {
		sess.Join("INNER", "user", "`repository`.owner_id = `user`.id")
	}
	sess.Where(cond).
		And(builder.Neq{field: ""}).
		GroupBy(field)

	if err := sess.Find(&rows); err != nil {
		return nil, fmt.Errorf("find: %v", err)
	}

	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.FieldValue] = row.Cnt
	}
	return counts, nil
}

// catalogStatsRow is the scan target of the aggregate stats query: the base
// CatalogStats columns plus the healthcheck counts, which only stats-ext exposes.
type catalogStatsRow struct {
	structs.CatalogStats    `xorm:"extends"`
	HealthcheckSuccessCount int64
	HealthcheckInfoCount    int64
	HealthcheckWarningCount int64
	HealthcheckErrorCount   int64
	NoHealthcheckCount      int64
}

// getCatalogStatsRow returns aggregate counts of the door43_metadata entries matching
// opts in a single query. The unique-value counts (lang, subject, flavor, owner, repo)
// are DISTINCT counts; the metadata-type, has_* and healthcheck counts are entry counts.
func getCatalogStatsRow(ctx context.Context, opts *door43metadata.SearchCatalogOptions) (*catalogStatsRow, error) {
	cond := door43metadata.SearchCatalogCondition(opts)
	condSQL, condArgs, err := builder.ToSQL(cond)
	if err != nil {
		return nil, err
	}

	// Translate table references to the aliases used in the raw query below
	condSQL = strings.ReplaceAll(condSQL, "`door43_metadata`.", "dm.")
	condSQL = strings.ReplaceAll(condSQL, "`repository`.", "r.")
	condSQL = strings.ReplaceAll(condSQL, "`user`.", "u.")
	condSQL = strings.ReplaceAll(condSQL, "`release`.", "rel.")

	// COUNT(DISTINCT CASE ...) ignores the NULLs of non-matching rows; COALESCE
	// guards the SUMs, which are NULL when no rows match.
	// Note: "release" is a MySQL reserved word and must be backtick-quoted in raw SQL.
	query := fmt.Sprintf(`SELECT
  COUNT(*) AS entry_count,
  COUNT(DISTINCT dm.language) AS lang_count,
  COUNT(DISTINCT CASE WHEN dm.language_direction = 'ltr' THEN dm.language END) AS lang_ltr_count,
  COUNT(DISTINCT CASE WHEN dm.language_direction = 'rtl' THEN dm.language END) AS lang_rtl_count,
  COUNT(DISTINCT dm.subject) AS subject_count,
  COUNT(DISTINCT CASE WHEN dm.flavor_type <> '' THEN dm.flavor_type END) AS flavor_type_count,
  COUNT(DISTINCT CASE WHEN dm.flavor <> '' THEN dm.flavor END) AS flavor_count,
  COUNT(DISTINCT r.owner_id) AS owner_count,
  COUNT(DISTINCT dm.repo_id) AS repo_count,
  COALESCE(SUM(CASE WHEN dm.metadata_type = 'ts' THEN 1 ELSE 0 END), 0) AS ts_count,
  COALESCE(SUM(CASE WHEN dm.metadata_type = 'tc' THEN 1 ELSE 0 END), 0) AS tc_count,
  COALESCE(SUM(CASE WHEN dm.metadata_type = 'rc' THEN 1 ELSE 0 END), 0) AS rc_count,
  COALESCE(SUM(CASE WHEN dm.metadata_type = 'sb' THEN 1 ELSE 0 END), 0) AS sb_count,
  COALESCE(SUM(CASE WHEN dm.has_pdf THEN 1 ELSE 0 END), 0) AS has_pdf,
  COALESCE(SUM(CASE WHEN dm.has_audio THEN 1 ELSE 0 END), 0) AS has_audio,
  COALESCE(SUM(CASE WHEN dm.has_video THEN 1 ELSE 0 END), 0) AS has_video,
  COALESCE(SUM(CASE WHEN dm.has_stream THEN 1 ELSE 0 END), 0) AS has_stream,
  COALESCE(SUM(CASE WHEN dm.has_other THEN 1 ELSE 0 END), 0) AS has_other,
  COALESCE(SUM(CASE WHEN dm.has_pdf OR dm.has_audio OR dm.has_video OR dm.has_stream OR dm.has_other THEN 1 ELSE 0 END), 0) AS has_attachment,
  COALESCE(SUM(CASE WHEN dm.healthcheck_severity = %d THEN 1 ELSE 0 END), 0) AS healthcheck_success_count,
  COALESCE(SUM(CASE WHEN dm.healthcheck_severity = %d THEN 1 ELSE 0 END), 0) AS healthcheck_info_count,
  COALESCE(SUM(CASE WHEN dm.healthcheck_severity = %d THEN 1 ELSE 0 END), 0) AS healthcheck_warning_count,
  COALESCE(SUM(CASE WHEN dm.healthcheck_severity = %d THEN 1 ELSE 0 END), 0) AS healthcheck_error_count,
  COALESCE(SUM(CASE WHEN dm.healthcheck_severity IS NULL OR dm.healthcheck_severity = 0 THEN 1 ELSE 0 END), 0) AS no_healthcheck_count
FROM door43_metadata dm
INNER JOIN repository r ON r.id = dm.repo_id
INNER JOIN user u ON r.owner_id = u.id
LEFT JOIN `+"`release`"+` rel ON rel.id = dm.release_id
WHERE `+condSQL,
		door43metadata.SeverityLevelSuccess,
		door43metadata.SeverityLevelInfo,
		door43metadata.SeverityLevelWarning,
		door43metadata.SeverityLevelError)

	row := &catalogStatsRow{}
	if _, err := db.GetEngine(ctx).SQL(query, condArgs...).Get(row); err != nil {
		return nil, fmt.Errorf("stats query: %v", err)
	}
	return row, nil
}

// GetCatalogStats returns the aggregate counts of the door43_metadata entries matching opts
func GetCatalogStats(ctx context.Context, opts *door43metadata.SearchCatalogOptions) (*structs.CatalogStats, error) {
	row, err := getCatalogStatsRow(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &row.CatalogStats, nil
}

// GetCatalogStatsExt returns GetCatalogStats plus the healthcheck counts and the entry
// counts per subject, flavor type, flavor, owner, language and metadata type.
func GetCatalogStatsExt(ctx context.Context, opts *door43metadata.SearchCatalogOptions) (*structs.CatalogStatsExt, error) {
	row, err := getCatalogStatsRow(ctx, opts)
	if err != nil {
		return nil, err
	}
	ext := &structs.CatalogStatsExt{
		CatalogStats:            row.CatalogStats,
		HealthcheckSuccessCount: row.HealthcheckSuccessCount,
		HealthcheckInfoCount:    row.HealthcheckInfoCount,
		HealthcheckWarningCount: row.HealthcheckWarningCount,
		HealthcheckErrorCount:   row.HealthcheckErrorCount,
		NoHealthcheckCount:      row.NoHealthcheckCount,
	}

	cond := door43metadata.SearchCatalogCondition(opts)
	fieldCounts := []struct {
		field string
		dest  *map[string]int64
	}{
		{"`door43_metadata`.subject", &ext.Subjects},
		{"`door43_metadata`.flavor_type", &ext.FlavorTypes},
		{"`door43_metadata`.flavor", &ext.Flavors},
		{"`user`.lower_name", &ext.Owners},
		{"`door43_metadata`.language", &ext.Languages},
		{"`door43_metadata`.metadata_type", &ext.MetadataTypes},
	}
	for _, fc := range fieldCounts {
		counts, err := SearchDoor43MetadataFieldCountsByCondition(ctx, cond, fc.field)
		if err != nil {
			return nil, err
		}
		*fc.dest = counts
	}
	return ext, nil
}

// SearchCatalogForBookPackage returns catalog repositories based on search options for a book package,
// it returns results in given range and number of total results.
func SearchCatalogForBookPackage(ctx context.Context, dm *repo.Door43Metadata, opts *door43metadata.SearchCatalogOptions) (repo.Door43MetadataList, int64, error) {
	books := opts.Books
	opts.Books = nil
	bookCond := builder.NewCond()

	if len(books) > 0 {
		innerBookCond := builder.NewCond()
		for _, book := range books {
			for v := range strings.SplitSeq(book, ",") {
				innerBookCond = innerBookCond.And(builder.Expr("JSON_SEARCH(dm.ingredients, 'one', ? COLLATE utf8mb4_general_ci, NULL, '$[*].identifier') IS NOT NULL", strings.ToLower(v)))
				// innerBookCond = innerBookCond.And(builder.Expr("JSON_CONTAINS(LOWER(JSON_EXTRACT(dm.ingredients, '$')), JSON_OBJECT('identifier', ?))", strings.ToLower(v)))
			}
		}
		bookCond = builder.Or(
			builder.In("`door43_metadata`.subject", []string{"Translation Academy", "Translation Words"}),
			innerBookCond,
		)
	}
	opts.Books = nil

	cond := door43metadata.SearchCatalogCondition(opts)
	cond = cond.And(bookCond)
	return SearchCatalogForBookPackageByCondition(ctx, dm, opts, cond)
}

// SearchCatalogForBookPackageByCondition search repositories by condition for a book package
func SearchCatalogForBookPackageByCondition(ctx context.Context, dm *repo.Door43Metadata, opts *door43metadata.SearchCatalogOptions, cond builder.Cond) (repo.Door43MetadataList, int64, error) {
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
WITH filtered AS (
  SELECT
    dm.*,
    ABS(dm.release_date_unix - ?) AS time_diff
  FROM door43_metadata dm
  JOIN repository r ON r.id = dm.repo_id
  WHERE
    dm.stage <= ?
    AND dm.language = ?
    AND r.owner_id = ?
    AND (` + condSQL + `)
),
ranked AS (
  SELECT
    f.*,
    ROW_NUMBER() OVER (
      PARTITION BY f.repo_id
      ORDER BY f.time_diff ASC, f.release_date_unix ASC
    ) AS rn
  FROM filtered f
)
SELECT 
  id, repo_id, release_id, ref, ref_type, commit_sha, stage,
  metadata_type, metadata_version, subject, flavor_type, flavor,
  abbreviation, title, publisher, language, language_title,
  language_direction, language_is_gl, content_format, checking_level,
  ingredients, relations, has_audio, has_video, has_pdf, has_stream, has_other,
  is_latest_for_stage, is_repo_metadata,
  metadata, validation_error, healthcheck_severity, healthcheck_counts,
  release_date_unix, created_unix, updated_unix
FROM ranked
WHERE rn = 1
ORDER BY IF(repo_id = ?, 1, 0) DESC, subject ASC`

	// Prepare query arguments: owner, repoName, ref, then all condition args
	queryArgs := []any{dm.ReleaseDateUnix, dm.Stage, dm.Language, dm.Repo.OwnerID}
	queryArgs = append(queryArgs, condArgs...)
	queryArgs = append(queryArgs, dm.Repo.ID)

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

// GetOrigLanguageBibles get the repositories for Hebrew and Greek Bibles with optional book filtering
func GetOrigLanguageBibles(ctx context.Context, dm *repo.Door43Metadata, books []string) (repo.Door43MetadataList, int64, error) {
	bookCond := door43metadata.GetBookCond(books)
	// Build the WHERE clause from the builder.Cond for use in the filtered CTE
	// We need to convert the builder conditions to SQL that can be used in the native query
	condSQL, condArgs, err := builder.ToSQL(bookCond)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build condition SQL: %v", err)
	}

	// Replace table aliases in the condition to match the CTE context
	// In the filtered CTE, we use 'dm' as the alias for door43_metadata table
	if condSQL != "" {
		condSQL = "AND (" + strings.ReplaceAll(condSQL, "`door43_metadata`.", "dm.") + ")"
	}

	// Build the complete query with CTEs
	query := `
WITH filtered AS (
  SELECT
    dm.*,
    ABS(dm.release_date_unix - ?) AS time_diff
  FROM door43_metadata dm
  JOIN repository r ON r.id = dm.repo_id
	JOIN user u ON r.owner_id = u.id
  WHERE
    dm.stage <= 2
    AND u.lower_name = 'unfoldingword'
		AND (r.lower_name = 'hbo_uhb' OR r.lower_name = 'el-x-koine_ugnt')
    ` + condSQL + `
),
ranked AS (
  SELECT
    f.*,
    ROW_NUMBER() OVER (
      PARTITION BY f.repo_id
      ORDER BY f.time_diff ASC, f.release_date_unix ASC
    ) AS rn
  FROM filtered f
)
SELECT 
  id, repo_id, release_id, ref, ref_type, commit_sha, stage,
  metadata_type, metadata_version, subject, flavor_type, flavor,
  abbreviation, title, publisher, language, language_title,
  language_direction, language_is_gl, content_format, checking_level,
  ingredients, relations, has_audio, has_video, has_pdf, has_stream, has_other,
  is_latest_for_stage, is_repo_metadata,
  metadata, validation_error, healthcheck_severity, healthcheck_counts,
  release_date_unix, created_unix, updated_unix
FROM ranked
WHERE rn = 1`

	queryArgs := []any{dm.ReleaseDateUnix}
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
