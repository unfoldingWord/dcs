// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
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
	CatalogOrderByTitle              CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title') ASC"
	CatalogOrderByTitleReverse       CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.title') DESC"
	CatalogOrderBySubject            CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject') ASC"
	CatalogOrderBySubjectReverse     CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.subject') DESC"
	CatalogOrderByIdentifier         CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.identifier') ASC"
	CatalogOrderByIdentifierReverse  CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.identifier') DESC"
	CatalogOrderByRepoName           CatalogOrderBy = "`repository`.lower_name ASC"
	CatalogOrderByRepoNameReverse    CatalogOrderBy = "`repository`.lower_name DESC"
	CatalogOrderByTag                CatalogOrderBy = "`door43_metadata`.branch_or_tag ASC"
	CatalogOrderByTagReverse         CatalogOrderBy = "`door43_metadata`.branch_or_tag DESC"
	CatalogOrderByReleaseDate        CatalogOrderBy = "`door43_metadata`.branch_or_tag ASC"
	CatalogOrderByReleaseDateReverse CatalogOrderBy = "`door43_metadata`.branch_or_tag DESC"
	CatalogOrderByLangCode           CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier') ASC"
	CatalogOrderByLangCodeReverse    CatalogOrderBy = "JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier') DESC"
	CatalogOrderByOldest             CatalogOrderBy = "`door43_metadata`.release_date_unix ASC"
	CatalogOrderByNewest             CatalogOrderBy = "`door43_metadata`.release_date_unix DESC"
	CatalogOrderByReleases           CatalogOrderBy = "release_count ASC"
	CatalogOrderByReleasesReverse    CatalogOrderBy = "release_count DESC"
	CatalogOrderByStars              CatalogOrderBy = "`repository`.num_stars ASC"
	CatalogOrderByStarsReverse       CatalogOrderBy = "`repository`.num_stars DESC"
	CatalogOrderByForks              CatalogOrderBy = "`repository`.num_forks ASC"
	CatalogOrderByForksReverse       CatalogOrderBy = "`repository`.num_forks DESC"
)

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

// GetMetadataCondByDBType Get the metadata condition by DB type
func GetMetadataCondByDBType(dbType, keyword string, includeMetadata bool) builder.Cond {
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
		keywordCond = keywordCond.Or(GetMetadataCondByDBType(setting.Database.Type, keyword, opts.IncludeMetadata))
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
				langCond = langCond.
					Or(builder.Like{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier'), '\"', ''))", strings.ToLower(v)}).
					Or(builder.Like{"CONCAT(SUBSTRING_INDEX(`repository`.lower_name, '_', 1), '_')", strings.ToLower(v) + "\\_"})
			} else {
				langCond = langCond.
					Or(builder.Eq{"LOWER(REPLACE(JSON_EXTRACT(`door43_metadata`.metadata, '$.dublin_core.language.identifier'), '\"', ''))": strings.ToLower(v)}).
					Or(builder.Eq{"CONCAT(SUBSTRING_INDEX(`repository`.lower_name, '_', 1), '_')": strings.ToLower(v) + "_"})
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
