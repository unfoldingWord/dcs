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
	CatalogOrderByTag             CatalogOrderBy = "CAST(TRIM(LEADING 'v' FROM `release`.tag_name) AS unsigned) ASC, `release`.tag_name ASC, `release`.created_unix ASC"
	CatalogOrderByTagReverse      CatalogOrderBy = "CAST(TRIM(LEADING 'v' FROM `release`.tag_name) AS unsigned) DESC, `release`.tag_name DESC, `release`.created_unix DESC"
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
	StageAll         string = "all"
	StageInvalid     string = "invalid"
)

// AllStages list of all stages
var AllStages = []string{StageProd, StagePreProd, StageDraft, StageLatest}

// SearchCatalogCondition creates a query condition according search repository options
func SearchCatalogCondition(opts *SearchCatalogOptions) builder.Cond {
	var repoCond, ownerCond builder.Cond
	if opts.RepoID > 0 {
		repoCond = builder.Eq{"`repository`.ID": opts.RepoID}
	} else {
		repoCond = GetRepoCond(opts.Repos)
		ownerCond = GetOwnerCond(opts.Owners)
	}

	keywordCond := builder.NewCond()
	for _, keyword := range opts.Keywords {
		keywordCond = keywordCond.Or(builder.Like{"`repository`.lower_name", strings.ToLower(keyword)})
		keywordCond = keywordCond.Or(builder.Like{"`user`.lower_name", strings.ToLower(keyword)})
		keywordCond = keywordCond.Or(builder.Like{"LOWER(JSON_UNQUOTE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title')))", strings.ToLower(keyword)})
		keywordCond = keywordCond.Or(builder.Like{"LOWER(JSON_UNQUOTE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject')))", strings.ToLower(keyword)})
		if opts.SearchAllMetadata {
			keywordCond = keywordCond.Or(builder.Expr("JSON_SEARCH(LOWER(`door43_metadata`.metadata), 'one', ?) IS NOT NULL", "%"+strings.ToLower(keyword)+"%"))
		}
	}

	stageCond, historyCond := getStageAndHistoryCond(opts.Stages, opts.IncludeHistory)

	cond := builder.NewCond().And(GetSubjectCond(opts.Subjects),
		GetBookCond(opts.Books),
		GetLanguageCond(opts.Languages),
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

	stages := FilterStages(opts.Stages)
	if contains(stages, StageProd) {
		sess.Join("LEFT", "(SELECT `release`.repo_id, COUNT(*) AS prod_count, MAX(`release`.created_unix) AS latest_prod_created_unix FROM `release` JOIN `door43_metadata` ON `door43_metadata`.release_id = `release`.id WHERE `release`.is_prerelease = 0 AND `release`.is_draft = 0 GROUP BY `release`.repo_id) `prod_info`", "`prod_info`.repo_id = `door43_metadata`.repo_id")
	}
	if contains(stages, StagePreProd) {
		sess.Join("LEFT", "(SELECT `release`.repo_id, COUNT(*) AS preprod_count, MAX(`release`.created_unix) AS latest_preprod_created_unix FROM `release` JOIN `door43_metadata` ON `door43_metadata`.release_id = `release`.id WHERE `release`.is_prerelease = 1 AND `release`.is_draft = 0 GROUP BY `release`.repo_id) `preprod_info`", "`preprod_info`.repo_id = `door43_metadata`.repo_id")
	}
	if contains(stages, StageDraft) {
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

// SplitAtCommaNotInString split s at commas, ignoring commas in strings.
func SplitAtCommaNotInString(s string, requireSpaceAfterComma bool) []string {
	var res []string
	var beg int
	var inString bool
	var prevIsComma bool

	for i := 0; i < len(s); i++ {
		if requireSpaceAfterComma && s[i] == ',' && !inString {
			prevIsComma = true
		} else if s[i] == ' ' && prevIsComma {
			res = append(res, strings.TrimSpace(s[beg:i-1]))
			beg = i + 1
		} else if !requireSpaceAfterComma && s[i] == ',' && !inString {
			res = append(res, strings.TrimSpace(s[beg:i]))
			beg = i + 1
		} else if s[i] == '"' {
			prevIsComma = false
			if !inString {
				inString = true
			} else if i > 0 && s[i-1] != '\\' {
				inString = false
			}
		}
	}
	return append(res, strings.TrimSpace(s[beg:]))
}

// FilterStages filters an array of strings to contain the right stages. If empty, returns StageProd in the array
func FilterStages(stages []string) []string {
	var filtered []string
	for _, stage := range stages {
		for _, v := range strings.Split(stage, ",") {
			switch v {
			case StageProd:
				if !contains(filtered, StageProd) {
					filtered = append(filtered, StageProd)
				}
			case StagePreProd, StagePreDashProd, StagePrerelease:
				if !contains(filtered, StagePreProd) {
					filtered = append(filtered, StagePreProd)
				}
			case StageDraft:
				if !contains(filtered, StageDraft) {
					filtered = append(filtered, StageDraft)
				}
			case StageLatest:
				if !contains(filtered, StageLatest) {
					filtered = append(filtered, StageLatest)
				}
			case StageAll:
				filtered = AllStages
				return filtered
			default:
				if !contains(filtered, StageInvalid) {
					filtered = append(filtered, StageInvalid)
				}
			}
		}
	}
	if len(filtered) == 0 {
		filtered = append(filtered, StageProd)
	}
	return filtered
}

func getStageAndHistoryCond(stages []string, includeHistory bool) (builder.Cond, builder.Cond) {
	stageCond := builder.NewCond()
	historyCond := builder.NewCond()
	for _, stage := range FilterStages(stages) {
		switch stage {
		case StageDraft:
			stageCond = stageCond.Or(builder.Eq{"`release`.is_draft": true})
			if !includeHistory {
				historyCond = historyCond.Or(builder.Expr("`release`.created_unix = latest_draft_created_unix"))
			}
		case StagePreProd, StagePreDashProd, StagePrerelease:
			stageCond = stageCond.Or(builder.Eq{"`release`.is_prerelease": true})
			if !includeHistory {
				historyCond = historyCond.Or(builder.Expr("`release`.created_unix = latest_preprod_created_unix"))
			}
		case StageLatest:
			stageCond = stageCond.Or(builder.Eq{"`door43_metadata`.release_id": 0})
			if !includeHistory {
				historyCond = historyCond.Or(builder.Expr("`release`.created_unix IS NULL"))
			}
		case StageProd:
			stageCond = stageCond.Or(builder.And(
				builder.Eq{"`release`.is_draft": false},
				builder.Eq{"`release`.is_prerelease": false},
				builder.Neq{"`door43_metadata`.release_id": 0}))
			if !includeHistory {
				historyCond = historyCond.Or(builder.Expr("`release`.created_unix = latest_prod_created_unix"))
			}
		case StageInvalid:
			stageCond = stageCond.Or(builder.Expr("0 = 1"))
		}
	}
	return stageCond, historyCond
}

// GetSubjectCond gets the subject condition
func GetSubjectCond(subjects []string) builder.Cond {
	var subjectCond = builder.NewCond()
	for _, subject := range subjects {
		subjectCond = subjectCond.Or(builder.Eq{"LOWER(JSON_UNQUOTE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject')))": strings.ToLower(subject)})
	}
	return subjectCond
}

// GetLanguageCond gets the laguage condition
func GetLanguageCond(languages []string) builder.Cond {
	var langCond = builder.NewCond()
	for _, lang := range languages {
		for _, v := range strings.Split(lang, ",") {
			langCond = langCond.Or(builder.Eq{"LOWER(JSON_UNQUOTE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier')))": strings.ToLower(v)})
		}
	}
	return langCond
}

// GetBookCond gets the book condition
func GetBookCond(books []string) builder.Cond {
	var bookCond = builder.NewCond()
	for _, book := range books {
		for _, v := range strings.Split(book, ",") {
			bookCond = bookCond.Or(builder.Expr("JSON_CONTAINS(LOWER(JSON_EXTRACT(`door43_metadata`.metadata, '$.projects')), JSON_OBJECT('identifier', ?))", strings.ToLower(v)))
		}
	}
	return bookCond
}

// GetCheckingLevelCond gets the checking level condition
func GetCheckingLevelCond(checkingLevels []string) builder.Cond {
	var checkingCond = builder.NewCond()
	for _, checking := range checkingLevels {
		for _, v := range strings.Split(checking, ",") {
			checkingCond = checkingCond.Or(builder.Eq{"JSON_UNQUOTE(JSON_EXTRACT(`door43_metadata`.metadata, '$.checking.checking_level'))": v})
		}
	}
	return checkingCond
}

// GetTagCond gets the tag condition
func GetTagCond(tags []string) builder.Cond {
	var tagCond = builder.NewCond()
	for _, tag := range tags {
		for _, v := range strings.Split(tag, ",") {
			tagCond = tagCond.Or(builder.Eq{"`release`.tag_name": v})
		}
	}
	return tagCond
}

// GetRepoCond gets the repo condition
func GetRepoCond(repos []string) builder.Cond {
	var repoCond = builder.NewCond()
	for _, repo := range repos {
		for _, v := range strings.Split(repo, ",") {
			repoCond = repoCond.Or(builder.Eq{"`repository`.lower_name": strings.ToLower(v)})
		}
	}
	return repoCond
}

// GetOwnerCond gets the owner condition
func GetOwnerCond(owners []string) builder.Cond {
	var ownerCond = builder.NewCond()
	for _, owner := range owners {
		for _, v := range strings.Split(owner, ",") {
			ownerCond = ownerCond.Or(builder.Eq{"`user`.lower_name": strings.ToLower(v)})
		}
	}
	return ownerCond
}
