// Copyright 202 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"xorm.io/builder"
)

//CatalogOrderBy is used to sort the result
type CatalogOrderBy string

func (s CatalogOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	CatalogOrderByTitle           CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title') ASC"
	CatalogOrderByTitleReverse    CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title') DESC"
	CatalogOrderBySubject         CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject') ASC"
	CatalogOrderBySubjectReverse  CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject') DESC"
	CatalogOrderByLangName        CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.langauge.title') ASC"
	CatalogOrderByLangNameReverse CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.title') DESC"
	CatalogOrderByLangCode        CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier') ASC"
	CatalogOrderByLangCodeReverse CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier') DESC"
	CatalogOrderByOldest          CatalogOrderBy = "`release_info`.latest_created_unix ASC"
	CatalogOrderByNewest          CatalogOrderBy = "`release_info`.latest_created_unix DESC"
	CatalogOrderByReleases        CatalogOrderBy = "`release_info`.num_releases ASC"
	CatalogOrderByReleasesReverse CatalogOrderBy = "`release_info`.num_releases DESC"
	CatalogOrderByStars           CatalogOrderBy = "`repository`.num_stars ASC"
	CatalogOrderByStarsReverse    CatalogOrderBy = "`repository`.num_stars DESC"
	CatalogOrderByForks           CatalogOrderBy = "`repository`.num_forks ASC"
	CatalogOrderByForksReverse    CatalogOrderBy = "`repository`.num_forks DESC"
)

// Door43MetadataListDefaultPageSize is the default number of repositories
// to load in memory when running administrative tasks on all (or almost
// all) of them.
// The number should be low enough to avoid filling up all RAM with
// repository data...
const Door43MetadataListDefaultPageSize = 64

// Door43MetadataList contains a list of repositories
type Door43MetadataList []*Door43Metadata

func (dms Door43MetadataList) Len() int {
	return len(dms)
}

func (dms Door43MetadataList) Less(i, j int) bool {
	return dms[i].Repo.FullName() < dms[j].Repo.FullName()
}

func (dms Door43MetadataList) Swap(i, j int) {
	dms[i], dms[j] = dms[j], dms[i]
}

// Door43MetadataListOfMap make list from values of map
func Door43MetadataListOfMap(dmMap map[int64]*Door43Metadata) Door43MetadataList {
	return Door43MetadataList(valuesDoor43Metadata(dmMap))
}

func (dms Door43MetadataList) loadAttributes(e Engine) error {
	if len(dms) == 0 {
		return nil
	}

	for _, dm := range dms {
		dm.loadAttributes(e)
	}

	return nil
}

// LoadAttributes loads the attributes for the given Door43MetadataList
func (dms Door43MetadataList) LoadAttributes() error {
	return dms.loadAttributes(x)
}

func valuesDoor43Metadata(m map[int64]*Door43Metadata) []*Door43Metadata {
	var values = make([]*Door43Metadata, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// SearchCatalogOptions holds the search options
type SearchCatalogOptions struct {
	ListOptions
	Keyword            string
	OrderBy            CatalogOrderBy
	TopicOnly          bool
	IncludeAllMetadata bool
}

// SearchCatalogCondition creates a query condition according search repository options
func SearchCatalogCondition(opts *SearchCatalogOptions) builder.Cond {
	var cond = builder.NewCond()
	cond = cond.And(
		builder.Eq{"`repository`.is_private": false},
		builder.Eq{"`repository`.is_archived": false},
		builder.Eq{"`release`.is_prerelease": false})
	if opts.Keyword != "" {
		// separate keyword
		var subQueryCond = builder.NewCond()
		for _, v := range strings.Split(opts.Keyword, ",") {
			if opts.TopicOnly {
				subQueryCond = subQueryCond.Or(builder.Eq{"topic.name": strings.ToLower(v)})
			} else {
				subQueryCond = subQueryCond.Or(builder.Like{"topic.name", strings.ToLower(v)})
			}
		}
		subQuery := builder.Select("repo_topic.repo_id").From("repo_topic").
			Join("INNER", "topic", "topic.id = repo_topic.topic_id").
			Where(subQueryCond).
			GroupBy("repo_topic.repo_id")
		var keywordCond = builder.In("`repository`.id", subQuery)
		if !opts.TopicOnly {
			var likes = builder.NewCond()
			for _, v := range strings.Split(opts.Keyword, ",") {
				likes = likes.Or(builder.Like{"LOWER(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title'))", strings.ToLower(v)})
				likes = likes.Or(builder.Like{"LOWER(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject'))", strings.ToLower(v)})
				if opts.IncludeAllMetadata {
					likes = likes.Or(builder.Expr("JSON_SEARCH(LOWER(`door43_metadata`.metadata), 'one', ?) IS NOT NULL", "%"+strings.ToLower(v)+"%"))
				}
			}
			keywordCond = keywordCond.Or(likes)
		}
		cond = cond.And(keywordCond)
	}

	return cond
}

// SearchCatalog returns catalog repositories based on search options,
// it returns results in given range and number of total results.
func SearchCatalog(opts *SearchCatalogOptions) (Door43MetadataList, int64, error) {
	cond := SearchCatalogCondition(opts)
	return SearchCatalogByCondition(opts, cond, true)
}

// SearchCatalogByCondition search repositories by condition
func SearchCatalogByCondition(opts *SearchCatalogOptions, cond builder.Cond, loadAttributes bool) (Door43MetadataList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = CatalogOrderByNewest
	}

	sess := x.NewSession()
	defer sess.Close()

	count, err := sess.
		Join("INNER", "release", "`release`.id = `door43_metadata`.release_id AND `release`.is_prerelease = 0").
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Where(cond).
		Count(new(Door43Metadata))

	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	dms := make(Door43MetadataList, 0, opts.PageSize)
	sess.
		Join("INNER", "(SELECT `release`.repo_id, COUNT(*) AS num_releases, MAX(`release`.created_unix) AS latest_created_unix FROM `release` JOIN `door43_metadata` ON `door43_metadata`.release_id = `release`.id WHERE `release`.is_prerelease = 0 GROUP BY `release`.repo_id) `release_info`", "`release_info`.repo_id = `door43_metadata`.repo_id").
		Join("INNER", "release", "`release`.id = `door43_metadata`.release_id AND `release`.created_unix = `release_info`.latest_created_unix AND `release`.is_prerelease = 0").
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Where(cond).
		OrderBy(opts.OrderBy.String())
	if opts.PageSize > 0 {
		sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	if err = sess.Find(&dms); err != nil {
		return nil, 0, fmt.Errorf("Find: %v", err)
	}

	if loadAttributes {
		if err = dms.loadAttributes(sess); err != nil {
			return nil, 0, fmt.Errorf("loadAttributes: %v", err)
		}
	}

	return dms, count, nil
}
