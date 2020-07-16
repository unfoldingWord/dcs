// Copyright 2020 unfoldingWord. All rights reserved.
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
	CatalogOrderByTag             CatalogOrderBy = "`release`.tag_name ASC"
	CatalogOrderByTagReverse      CatalogOrderBy = "`release`.tag_name DESC"
	CatalogOrderByLangCode        CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier') ASC"
	CatalogOrderByLangCodeReverse CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier') DESC"
	CatalogOrderByOldest          CatalogOrderBy = "`release`.created_unix ASC"
	CatalogOrderByNewest          CatalogOrderBy = "`release`.created_unix DESC"
	CatalogOrderByReleases        CatalogOrderBy = "prod_count ASC"
	CatalogOrderByReleasesReverse CatalogOrderBy = "prod_count DESC"
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
	RepoID            int64
	Keywords          []string
	Owners            []string
	Repos             []string
	Tags              []string
	Stages            []string
	Subjects          []string
	CheckingLevels    []string
	Books             []string
	IncludeHistory    bool
	SearchAllMetadata bool
	ShowIngredients   bool
	Languages         []string
	OrderBy           []CatalogOrderBy
}

// Stage values
const (
	StageProd        string = "prod"
	StagePreProd     string = "preprod"
	StagePreDashProd string = "pre-prod"
	StagePrerelease  string = "prerelease"
	StageDraft       string = "draft"
	StageLatest      string = "latest"
)

// SearchCatalogCondition creates a query condition according search repository options
func SearchCatalogCondition(opts *SearchCatalogOptions) builder.Cond {
	var cond = builder.NewCond()
	cond = cond.And(builder.Eq{"`repository`.is_private": false},
		builder.Eq{"`repository`.is_archived": false})

	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"`repository`.ID": opts.RepoID})
	} else {
		if len(opts.Repos) > 0 {
			var repoCond = builder.NewCond()
			for _, repo := range opts.Repos {
				repoCond = repoCond.Or(builder.Eq{"`repository`.lower_name": strings.ToLower(repo)})
			}
			cond.And(repoCond)
		}
		if len(opts.Owners) > 0 {
			var ownerCond = builder.NewCond()
			for _, owner := range opts.Owners {
				ownerCond = ownerCond.Or(builder.Eq{"`user`.lower_name": strings.ToLower(owner)})
			}
			cond.And(ownerCond)
		}
	}

	if len(opts.Stages) == 0 {
		cond = cond.And(builder.Eq{"`release`.is_prerelease": false}, builder.Eq{"`release`.is_draft": false})
		if !opts.IncludeHistory {
			cond = cond.And(builder.Expr("`release`.created_unix = latest_prod_created_unix"))
		}
	} else {
		var subStageCond = builder.NewCond()
		var subHistoryCond = builder.NewCond()
		for _, stage := range opts.Stages {
			switch stage {
			case StageDraft:
				subStageCond = subStageCond.Or(builder.Eq{"`release`.is_draft": true})
				if !opts.IncludeHistory {
					subHistoryCond = subHistoryCond.Or(builder.Expr("`release`.created_unix = latest_draft_created_unix"))
				}
			case StagePreProd, StagePreDashProd, StagePrerelease:
				subStageCond = subStageCond.Or(builder.Eq{"`release`.is_prerelease": true})
				if ! opts.IncludeHistory {
					subHistoryCond = subHistoryCond.Or(builder.Expr("`release`.created_unix = latest_preprod_created_unix"))
				}
			case StageLatest:
				subStageCond = subStageCond.Or(builder.Eq{"`door43_metadata`.release_id": 0})
				if ! opts.IncludeHistory {
					subHistoryCond = subHistoryCond.Or(builder.Expr("`release`.created_unix IS NULL"))
				}
			case StageProd:
				subStageCond = subStageCond.Or(builder.And(
					builder.Eq{"`release`.is_draft": false},
					builder.Eq{"`release`.is_prerelease": false},
					builder.Neq{"`door43_metadata`.release_id": 0}))
				if !opts.IncludeHistory {
					subHistoryCond = subHistoryCond.Or(builder.Expr("`release`.created_unix = latest_prod_created_unix"))
				}
			}
		}
		cond = cond.And(subStageCond).And(subHistoryCond)
	}

	if len(opts.Subjects) > 0 {
		var subjectCond = builder.NewCond()
		for _, subject := range opts.Subjects {
			subjectCond = subjectCond.Or(builder.Like{"LOWER(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject'))", strings.ToLower(subject)})
		}
		cond = cond.And(subjectCond)
	}
	if len(opts.Languages) > 0 {
		var langCond = builder.NewCond()
		for _, lang := range opts.Languages {
			// separate languages in case they used a comma
			for _, v := range strings.Split(lang, ",") {
				langCond = langCond.Or(builder.Like{"LOWER(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier'))", strings.ToLower(v)})
			}
		}
		cond = cond.And(langCond)
	}
	if len(opts.Books) > 0 {
		var bookCond = builder.NewCond()
		for _, book := range opts.Books {
			// separate books in case they used a comma
			for _, v := range strings.Split(book, ",") {
				bookCond = bookCond.Or(builder.Expr("JSON_CONTAINS(LOWER(JSON_EXTRACT(`door43_metadata`.metadata, '$.projects')), JSON_OBJECT('identifier', ?))", strings.ToLower(v)))
			}
		}
		cond = cond.And(bookCond)
	}
	if len(opts.CheckingLevels) > 0 {
		var checkingCond = builder.NewCond()
		for _, checking := range opts.CheckingLevels {
			checkingCond = checkingCond.Or(builder.Eq{"JSON_EXTRACT(`door43_metadata`.metadata, '$.checking.checking_level')": checking})
		}
		cond.And(checkingCond)
	}
	if len(opts.Tags) > 0 {
		cond = cond.And(builder.In("`release`.tag_name", opts.Tags))
	}

	if len(opts.Keywords) > 0 {
		keywordCond := builder.NewCond()
		for _, keyword := range opts.Keywords {
			keywordCond = keywordCond.Or(builder.Like{"`repository`.lower_name", strings.ToLower(keyword)})
			keywordCond = keywordCond.Or(builder.Like{"`user`.lower_name", strings.ToLower(keyword)})
			keywordCond = keywordCond.Or(builder.Like{"LOWER(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title'))", strings.ToLower(keyword)})
			keywordCond = keywordCond.Or(builder.Like{"LOWER(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject'))", strings.ToLower(keyword)})
			if opts.SearchAllMetadata {
				keywordCond = keywordCond.Or(builder.Expr("JSON_SEARCH(LOWER(`door43_metadata`.metadata), 'one', ?) IS NOT NULL", "%"+strings.ToLower(keyword)+"%"))
			}
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
		opts.OrderBy = []CatalogOrderBy{CatalogOrderByNewest}
	}

	sess := x.NewSession()
	defer sess.Close()

	dms := make(Door43MetadataList, 0, opts.PageSize)
	sess.Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Join("INNER", "user", "`repository`.owner_id = `user`.id").
		Join("LEFT", "release", "`release`.id = `door43_metadata`.release_id").
		Where(cond)

	for _, orderBy := range opts.OrderBy {
		sess.OrderBy(orderBy.String())
	}

	if len(opts.Stages) == 0 || contains(opts.Stages, "prod") {
		sess.Join("LEFT", "(SELECT `release`.repo_id, COUNT(*) AS prod_count, MAX(`release`.created_unix) AS latest_prod_created_unix FROM `release` JOIN `door43_metadata` ON `door43_metadata`.release_id = `release`.id WHERE `release`.is_prerelease = 0 GROUP BY `release`.repo_id) `prod_info`", "`prod_info`.repo_id = `door43_metadata`.repo_id")
	}
	if contains(opts.Stages, StagePreDashProd) || contains(opts.Stages, StagePreProd) || contains(opts.Stages, StagePrerelease) {
		sess.Join("LEFT", "(SELECT `release`.repo_id, COUNT(*) AS preprod_count, MAX(`release`.created_unix) AS latest_preprod_created_unix FROM `release` JOIN `door43_metadata` ON `door43_metadata`.release_id = `release`.id WHERE `release`.is_prerelease = 1 AND `release`.is_draft = 0 GROUP BY `release`.repo_id) `preprod_info`", "`preprod_info`.repo_id = `door43_metadata`.repo_id")
	}
	if contains(opts.Stages, StageDraft) {
		sess.Join("LEFT", "(SELECT `release`.repo_id, COUNT(*) AS draft_count, MAX(`release`.created_unix) AS latest_draft_created_unix FROM `release` JOIN `door43_metadata` ON `door43_metadata`.release_id = `release`.id WHERE `release`.is_draft = 1 GROUP BY `release`.repo_id) `draft_info`", "`draft_info`.repo_id = `door43_metadata`.repo_id")
	}
	
	if opts.PageSize > 0 {
		sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	count, err := sess.FindAndCount(&dms)
	if err != nil {
		return nil, 0, fmt.Errorf("FindAndCount: %v", err)
	}

	if loadAttributes {
		if err = dms.loadAttributes(sess); err != nil {
			return nil, 0, fmt.Errorf("loadAttributes: %v", err)
		}
	}

	return dms, count, nil
}
