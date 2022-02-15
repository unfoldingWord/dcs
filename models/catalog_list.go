// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// CatalogOrderBy is used to sort the result
type CatalogOrderBy string

func (s CatalogOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	CatalogOrderByTitle             CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title') ASC"
	CatalogOrderByTitleReverse      CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title') DESC"
	CatalogOrderBySubject           CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject') ASC"
	CatalogOrderBySubjectReverse    CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject') DESC"
	CatalogOrderByIdentifier        CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.identifier') ASC"
	CatalogOrderByIdentifierReverse CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.identifier') DESC"
	CatalogOrderByRepoName          CatalogOrderBy = "`repository`.lower_name ASC"
	CatalogOrderByRepoNameReverse   CatalogOrderBy = "`repository`.lower_name DESC"
	CatalogOrderByTag               CatalogOrderBy = "CAST(TRIM(LEADING 'v' FROM `release`.tag_name) AS unsigned) ASC, `door43_metadata`.branch_or_tag ASC, `door43_metadata`.release_date_unix ASC"
	CatalogOrderByTagReverse        CatalogOrderBy = "CAST(TRIM(LEADING 'v' FROM `release`.tag_name) AS unsigned) DESC, `door43_metadata`.branch_or_tag DESC, `door43_metadata`.release_date_unix DESC"
	CatalogOrderByLangCode          CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier') ASC"
	CatalogOrderByLangCodeReverse   CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier') DESC"
	CatalogOrderByOldest            CatalogOrderBy = "`door43_metadata`.release_date_unix ASC"
	CatalogOrderByNewest            CatalogOrderBy = "`door43_metadata`.release_date_unix DESC"
	CatalogOrderByReleases          CatalogOrderBy = "release_count ASC"
	CatalogOrderByReleasesReverse   CatalogOrderBy = "release_count DESC"
	CatalogOrderByStars             CatalogOrderBy = "`repository`.num_stars ASC"
	CatalogOrderByStarsReverse      CatalogOrderBy = "`repository`.num_stars DESC"
	CatalogOrderByForks             CatalogOrderBy = "`repository`.num_forks ASC"
	CatalogOrderByForksReverse      CatalogOrderBy = "`repository`.num_forks DESC"
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

// LoadAttributes loads the attributes for the given Door43MetadataList
func (dms Door43MetadataList) LoadAttributes() error {
	return dms.loadAttributes(db.GetEngine(db.DefaultContext))
}

func (dms Door43MetadataList) loadAttributes(e db.Engine) error {
	if len(dms) == 0 {
		return nil
	}
	var lastErr error
	for _, dm := range dms {
		if err := dm.loadAttributes(e); err != nil && lastErr == nil {
			lastErr = err
		}
	}
	return lastErr
}

func valuesDoor43Metadata(m map[int64]*Door43Metadata) []*Door43Metadata {
	values := make([]*Door43Metadata, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// SearchCatalogOptions holds the search options
type SearchCatalogOptions struct {
	db.ListOptions
	RepoID          int64
	Keywords        []string
	Owners          []string
	Repos           []string
	Tags            []string
	Stage           Stage
	Subjects        []string
	CheckingLevels  []string
	Books           []string
	IncludeHistory  bool
	IncludeMetadata bool
	ShowIngredients bool
	Languages       []string
	OrderBy         []CatalogOrderBy
	PartialMatch    bool
}

func getMetadataCondByDBType(dbType string, keyword string, includeMetadata bool) builder.Cond {
	cond := builder.NewCond()
	if dbType == "mysql" || dbType == "sqlite3" {
		cond = cond.Or(builder.Like{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title'), '\"', ''))", strings.ToLower(keyword)})
		cond = cond.Or(builder.Like{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject'), '\"', ''))", strings.ToLower(keyword)})
		if includeMetadata {
			if dbType == "mysql" {
				cond = cond.Or(builder.Expr("JSON_SEARCH(LOWER(`door43_metadata`.metadata), 'one', ?) IS NOT NULL", "%"+strings.ToLower(keyword)+"%"))
			} else {
				cond = cond.Or(builder.Like{"`door43_metadata`.metadata", `": "%` + strings.ToLower(keyword) + `%"`})
			}
		}
	} else {
		cond = cond.Or(builder.Like{"`door43_metadata`.metadata", `"title": "%` + strings.ToLower(keyword) + `%"`})
		cond = cond.Or(builder.Like{"`door43_metadata`.metadata", `"subject": "%` + strings.ToLower(keyword) + `%"`})
		if includeMetadata {
			cond = cond.Or(builder.Like{"`door43_metadata`.metadata", `": "%` + strings.ToLower(keyword) + `%"`})
		}
	}
	return cond
}

// SearchCatalogCondition creates a query condition according search repository options
func SearchCatalogCondition(opts *SearchCatalogOptions) builder.Cond {
	var repoCond, ownerCond builder.Cond
	if opts.RepoID > 0 {
		repoCond = builder.Eq{"`repository`.ID": opts.RepoID}
	} else {
		repoCond = GetRepoCond(opts.Repos, opts.PartialMatch)
		ownerCond = GetOwnerCond(opts.Owners, opts.PartialMatch)
	}

	keywordCond := builder.NewCond()
	for _, keyword := range opts.Keywords {
		keywordCond = keywordCond.Or(builder.Like{"`repository`.lower_name", strings.ToLower(keyword)})
		keywordCond = keywordCond.Or(builder.Like{"`user`.lower_name", strings.ToLower(keyword)})
		keywordCond = keywordCond.Or(getMetadataCondByDBType(setting.Database.Type, keyword, opts.IncludeMetadata))
	}

	stageCond := GetStageCond(opts.Stage)
	historyCond := GetHistoryCond(opts.IncludeHistory)

	cond := builder.NewCond().And(GetSubjectCond(opts.Subjects, opts.PartialMatch),
		GetBookCond(opts.Books),
		GetLanguageCond(opts.Languages, opts.PartialMatch),
		GetCheckingLevelCond(opts.CheckingLevels),
		GetTagCond(opts.Tags),
		repoCond,
		ownerCond,
		stageCond,
		historyCond,
		keywordCond,
		builder.Eq{"`repository`.is_private": false},
		builder.Eq{"`repository`.is_archived": false})

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
	return searchCatalogByCondition(db.GetEngine(db.DefaultContext), opts, cond, loadAttributes)
}

func searchCatalogByCondition(e db.Engine, opts *SearchCatalogOptions, cond builder.Cond, loadAttributes bool) (Door43MetadataList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize < 0 {
		opts.PageSize = 0
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = []CatalogOrderBy{CatalogOrderByNewest}
	}

	var dms Door43MetadataList
	if opts.PageSize > 0 {
		dms = make(Door43MetadataList, 0, opts.PageSize)
	}

	releaseInfoInner, err := builder.Select("`door43_metadata`.repo_id", "COUNT(*) AS release_count", "MAX(`door43_metadata`.release_date_unix) AS latest_unix").
		From("door43_metadata").
		GroupBy("`door43_metadata`.repo_id").
		Where(builder.Gt{"`door43_metadata`.release_date_unix": 0}).
		Where(GetStageCond(opts.Stage)).
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
		if err = dms.loadAttributes(sess); err != nil {
			return nil, 0, fmt.Errorf("loadAttributes: %v", err)
		}
	}

	return dms, count, nil
}

// SplitAtCommaNotInString split s at commas, ignoring commas in strings.
func SplitAtCommaNotInString(s string, requireSpaceAfterComma bool) []string {
	var res []string
	var beg int
	var inString bool
	var prevIsComma bool

	for i := 0; i < len(s); i++ {
		if requireSpaceAfterComma && s[i] == ',' && !inString {
			prevIsComma = true
			continue
		} else if s[i] == ' ' {
			if prevIsComma {
				res = append(res, strings.TrimSpace(s[beg:i-1]))
				beg = i + 1
			} else {
				continue
			}
		} else if !requireSpaceAfterComma && s[i] == ',' && !inString {
			res = append(res, strings.TrimSpace(s[beg:i]))
			beg = i + 1
		} else if s[i] == '"' {
			if !inString {
				inString = true
			} else if i > 0 && s[i-1] != '\\' {
				inString = false
			}
		}
		prevIsComma = false
	}
	return append(res, strings.TrimSpace(s[beg:]))
}

// GetStageCond gets the condition for the given stage
func GetStageCond(stage Stage) builder.Cond {
	return builder.Lte{"`door43_metadata`.stage": stage}
}

// GetHistoryCond gets the conditions if IncludeHistory is false
func GetHistoryCond(includeHistory bool) builder.Cond {
	if includeHistory {
		return nil
	}
	return builder.And(builder.Expr("`door43_metadata`.release_date_unix = latest_unix"), builder.Expr("`door43_metadata`.stage = latest_stage"))
}

// GetSubjectCond gets the subject condition
func GetSubjectCond(subjects []string, partialMatch bool) builder.Cond {
	subjectCond := builder.NewCond()
	for _, subject := range subjects {
		if partialMatch {
			subjectCond = subjectCond.Or(builder.Like{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject'), '\"', ''))", strings.ToLower(subject)})
		} else {
			subjectCond = subjectCond.Or(builder.Eq{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject'), '\"', ''))": strings.ToLower(subject)})
		}
	}
	return subjectCond
}

// GetLanguageCond gets the language condition
func GetLanguageCond(languages []string, partialMatch bool) builder.Cond {
	langCond := builder.NewCond()
	for _, lang := range languages {
		for _, v := range strings.Split(lang, ",") {
			if partialMatch {
				langCond = langCond.Or(builder.Like{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier'), '\"', ''))", strings.ToLower(v)})
			} else {
				langCond = langCond.Or(builder.Eq{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier'), '\"', ''))": strings.ToLower(v)})
			}
		}
	}
	return langCond
}

// GetBookCond gets the book condition
func GetBookCond(books []string) builder.Cond {
	bookCond := builder.NewCond()
	for _, book := range books {
		for _, v := range strings.Split(book, ",") {
			bookCond = bookCond.Or(builder.Expr("JSON_CONTAINS(LOWER(JSON_EXTRACT(`door43_metadata`.metadata, '$.projects')), JSON_OBJECT('identifier', ?))", strings.ToLower(v)))
		}
	}
	return bookCond
}

// GetCheckingLevelCond gets the checking level condition
func GetCheckingLevelCond(checkingLevels []string) builder.Cond {
	checkingCond := builder.NewCond()
	for _, checking := range checkingLevels {
		for _, v := range strings.Split(checking, ",") {
			checkingCond = checkingCond.Or(builder.Gte{"REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.checking.checking_level'), '\"', '')": v})
		}
	}
	return checkingCond
}

// GetTagCond gets the tag condition
func GetTagCond(tags []string) builder.Cond {
	tagCond := builder.NewCond()
	for _, tag := range tags {
		for _, v := range strings.Split(tag, ",") {
			tagCond = tagCond.Or(builder.Eq{"`release`.tag_name": v})
		}
	}
	return tagCond
}

// GetRepoCond gets the repo condition
func GetRepoCond(repos []string, partialMatch bool) builder.Cond {
	repoCond := builder.NewCond()
	for _, repo := range repos {
		for _, v := range strings.Split(repo, ",") {
			if partialMatch {
				repoCond = repoCond.Or(builder.Like{"`repository`.lower_name", strings.ToLower(v)})
			} else {
				repoCond = repoCond.Or(builder.Eq{"`repository`.lower_name": strings.ToLower(v)})
			}
		}
	}
	return repoCond
}

// GetOwnerCond gets the owner condition
func GetOwnerCond(owners []string, partialMatch bool) builder.Cond {
	ownerCond := builder.NewCond()
	for _, owner := range owners {
		for _, v := range strings.Split(owner, ",") {
			if partialMatch {
				ownerCond = ownerCond.Or(builder.Like{"`user`.lower_name", strings.ToLower(v)})
			} else {
				ownerCond = ownerCond.Or(builder.Eq{"`user`.lower_name": strings.ToLower(v)})
			}
		}
	}
	return ownerCond
}

// QueryForCatalogV3 Does a special query for all of V3
func QueryForCatalogV3(opts *SearchCatalogOptions) (Door43MetadataList, error) {
	return queryForCatalogV3(db.GetEngine(db.DefaultContext), opts)
}

func queryForCatalogV3(e db.Engine, opts *SearchCatalogOptions) (Door43MetadataList, error) {
	sess := e.Table(&Door43Metadata{}).
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Join("INNER", "user", "`repository`.owner_id = `user`.id").
		Where("(`door43_metadata`.stage = 0 AND `door43_metadata`.repo_id NOT IN (SELECT dm1.repo_id FROM door43_metadata dm1 INNER JOIN repository r1 ON dm1.repo_id = r1.id INNER JOIN user u1 ON u1.id = r1.owner_id WHERE u1.lower_name = \"door43-catalog\" AND dm1.stage = 3)) OR (`door43_metadata`.stage = 3 AND `user`.lower_name = \"door43-catalog\")").
		And("`door43_metadata`.stage = 3 OR `door43_metadata`.release_date_unix = (SELECT release_date_unix FROM (SELECT dm2.repo_id, dm2.stage, MAX(dm2.release_date_unix) AS release_date_unix FROM door43_metadata dm2 GROUP BY repo_id, dm2.stage ORDER BY dm2.stage) t WHERE `door43_metadata`.repo_id = t.repo_id LIMIT 1)").
		Where(GetSubjectCond(opts.Subjects, opts.PartialMatch)).
		Where(GetOwnerCond(opts.Owners, opts.PartialMatch)).
		Where(GetRepoCond(opts.Repos, opts.PartialMatch)).
		Where(GetLanguageCond(opts.Languages, opts.PartialMatch))

	for _, orderBy := range opts.OrderBy {
		sess.OrderBy(orderBy.String())
	}

	sess.OrderBy(string(CatalogOrderByLangCode)).
		OrderBy(string(CatalogOrderByIdentifier)).
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
