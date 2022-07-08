// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"

	"xorm.io/builder"
)

// SearchCatalog returns catalog repositories based on search options,
// it returns results in given range and number of total results.
func SearchCatalog(opts *door43metadata.SearchCatalogOptions) (Door43MetadataList, int64, error) {
	cond := door43metadata.SearchCatalogCondition(opts)
	return SearchCatalogByCondition(opts, cond, true)
}

// SearchCatalogByCondition search repositories by condition
func SearchCatalogByCondition(opts *door43metadata.SearchCatalogOptions, cond builder.Cond, loadAttributes bool) (Door43MetadataList, int64, error) {
	return searchCatalogByCondition(db.DefaultContext, opts, cond, loadAttributes)
}

func searchCatalogByCondition(ctx context.Context, opts *door43metadata.SearchCatalogOptions, cond builder.Cond, loadAttributes bool) (Door43MetadataList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize < 0 {
		opts.PageSize = 0
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = []door43metadata.CatalogOrderBy{door43metadata.CatalogOrderByNewest}
	}

	var dms Door43MetadataList
	if opts.PageSize > 0 {
		dms = make(Door43MetadataList, 0, opts.PageSize)
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

	if loadAttributes {
		if err = dms.loadAttributes(ctx); err != nil {
			return nil, 0, fmt.Errorf("loadAttributes: %v", err)
		}
	}

	return dms, count, nil
}

// QueryForCatalogV3 Does a special query for all of V3
func QueryForCatalogV3(opts *door43metadata.SearchCatalogOptions) (Door43MetadataList, error) {
	return queryForCatalogV3(db.GetEngine(db.DefaultContext), opts)
}

func queryForCatalogV3(e db.Engine, opts *door43metadata.SearchCatalogOptions) (Door43MetadataList, error) {
	sess := e.Table(&Door43Metadata{}).
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Join("INNER", "user", "`repository`.owner_id = `user`.id").
		Where("(`door43_metadata`.stage = 0 AND `door43_metadata`.repo_id NOT IN (SELECT dm1.repo_id FROM door43_metadata dm1 INNER JOIN repository r1 ON dm1.repo_id = r1.id INNER JOIN user u1 ON u1.id = r1.owner_id WHERE u1.lower_name = \"door43-catalog\" AND dm1.stage = 3)) OR (`door43_metadata`.stage = 3 AND `user`.lower_name = \"door43-catalog\")").
		And("`door43_metadata`.stage = 3 OR `door43_metadata`.release_date_unix = (SELECT release_date_unix FROM (SELECT dm2.repo_id, dm2.stage, MAX(dm2.release_date_unix) AS release_date_unix FROM door43_metadata dm2 GROUP BY repo_id, dm2.stage ORDER BY dm2.stage) t WHERE `door43_metadata`.repo_id = t.repo_id LIMIT 1)").
		Where(door43metadata.GetSubjectCond(opts.Subjects, opts.PartialMatch)).
		Where(door43metadata.GetOwnerCond(opts.Owners, opts.PartialMatch)).
		Where(door43metadata.GetRepoCond(opts.Repos, opts.PartialMatch)).
		Where(door43metadata.GetLanguageCond(opts.Languages, opts.PartialMatch))

	for _, orderBy := range opts.OrderBy {
		sess.OrderBy(orderBy.String())
	}

	sess.OrderBy(string(door43metadata.CatalogOrderByLangCode)).
		OrderBy(string(door43metadata.CatalogOrderByIdentifier)).
		OrderBy("IF(`user`.lower_name = \"door43-catalog\", 0, 1) ASC").
		OrderBy("IF(`user`.lower_name = \"unfoldingword\", 0, 1) ASC")

	var dms Door43MetadataList

	err := sess.Find(&dms)
	fmt.Println(sess.LastSQL())
	if err != nil {
		return nil, fmt.Errorf("FindAndCount: %v", err)
	}

	// Filter for unique language/resource combinations
	var filteredDMs Door43MetadataList
	for i, dm := range dms {
		if err := dm.LoadAttributes(); err != nil {
			return nil, err
		}
		unique := false
		for j := 0; j < i; j++ {
			if dms[j].Repo.LowerName == dm.Repo.LowerName {
				unique = true
			}
		}
		if unique {
			filteredDMs = append(filteredDMs, dm)
		}
	}

	// if err = dms.loadAttributes(sess); err != nil {
	// 	return nil, 0, fmt.Errorf("loadAttributes: %v", err)
	// }

	return filteredDMs, nil
}
