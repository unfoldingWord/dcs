// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// CatalogOrderBy is used to sort the result
type CatalogOrderBy string

func (s CatalogOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	CatalogOrderByTitle               CatalogOrderBy = "`door43_metadata`.title ASC"
	CatalogOrderByTitleReverse        CatalogOrderBy = "`door43_metadata`.title DESC"
	CatalogOrderBySubject             CatalogOrderBy = "`door43_metadata`.subject ASC"
	CatalogOrderBySubjectReverse      CatalogOrderBy = "`door43_metadata`.subject DESC"
	CatalogOrderByFlavorType          CatalogOrderBy = "`door43_metadata`.flavor_type ASC"
	CatalogOrderByFlavorTypeReverse   CatalogOrderBy = "`door43_metadata`.flavor_type DESC"
	CatalogOrderByFlavor              CatalogOrderBy = "`door43_metadata`.flavor ASC"
	CatalogOrderByFlavorReverse       CatalogOrderBy = "`door43_metadata`.flavor DESC"
	CatalogOrderByAbbreviation        CatalogOrderBy = "`door43_metadata`.abbreviation ASC"
	CatalogOrderByAbbreviationReverse CatalogOrderBy = "`door43_metadata`.abbreviation DESC"
	CatalogOrderByRepoName            CatalogOrderBy = "`repository`.lower_name ASC"
	CatalogOrderByRepoNameReverse     CatalogOrderBy = "`repository`.lower_name DESC"
	CatalogOrderByTag                 CatalogOrderBy = "`door43_metadata`.ref ASC"
	CatalogOrderByTagReverse          CatalogOrderBy = "`door43_metadata`.ref DESC"
	CatalogOrderByReleaseDate         CatalogOrderBy = "`door43_metadata`.ref ASC"
	CatalogOrderByReleaseDateReverse  CatalogOrderBy = "`door43_metadata`.ref DESC"
	CatalogOrderByLangCode            CatalogOrderBy = "`door43_metadata`.language ASC"
	CatalogOrderByLangCodeReverse     CatalogOrderBy = "`door43_metadata`.language DESC"
	CatalogOrderByOldest              CatalogOrderBy = "`door43_metadata`.release_date_unix ASC"
	CatalogOrderByNewest              CatalogOrderBy = "`door43_metadata`.release_date_unix DESC"
	CatalogOrderByReleases            CatalogOrderBy = "release_count ASC"
	CatalogOrderByReleasesReverse     CatalogOrderBy = "release_count DESC"
	CatalogOrderByStars               CatalogOrderBy = "`repository`.num_stars ASC"
	CatalogOrderByStarsReverse        CatalogOrderBy = "`repository`.num_stars DESC"
	CatalogOrderByForks               CatalogOrderBy = "`repository`.num_forks ASC"
	CatalogOrderByForksReverse        CatalogOrderBy = "`repository`.num_forks DESC"
)

// SearchCatalogOptions holds the search options
type SearchCatalogOptions struct {
	db.ListOptions
	RepoID           int64
	Keywords         []string
	Owners           []string
	Repos            []string
	Tags             []string
	Stage            Stage
	Subjects         []string
	FlavorTypes      []string
	Flavors          []string
	Abbreviations    []string
	ContentFormats   []string
	CheckingLevels   []string
	Books            []string
	IncludeHistory   bool
	MetadataTypes    []string
	MetadataVersions []string
	ShowIngredients  util.OptionalBool
	Languages        []string
	LanguageIsGL     util.OptionalBool
	OrderBy          []CatalogOrderBy
	PartialMatch     bool
}

// GetMetadataCond Get the metadata condition
func GetMetadataCond(keyword string) builder.Cond {
	cond := builder.NewCond()
	cond = cond.And(builder.Like{"`door43_metadata`.title", keyword})
	cond = cond.Or(builder.Eq{"`door43_metadata`.abbreviation": keyword})
	cond = cond.Or(builder.Like{"`door43_metadata`.subject", keyword})
	cond = cond.Or(builder.Eq{"`door43_metadata`.language": keyword})
	cond = cond.Or(builder.Like{"`door43_metadata`.language_title", keyword})
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
		keywordCond = keywordCond.Or(builder.Like{"`repository`.lower_name", strings.TrimSpace(keyword)})
		keywordCond = keywordCond.Or(builder.Like{"`user`.lower_name", strings.TrimSpace(keyword)})
		keywordCond = keywordCond.Or(GetMetadataCond(keyword))
	}

	stageCond := GetStageCond(opts.Stage)
	historyCond := GetHistoryCond(opts.IncludeHistory)

	langIsGLCond := builder.NewCond()
	if opts.LanguageIsGL != util.OptionalBoolNone {
		langIsGLCond = builder.Eq{"`door43_metadata`.language_is_gl": opts.LanguageIsGL.IsTrue()}
	}

	cond := builder.NewCond().And(
		GetSubjectCond(opts.Subjects, opts.PartialMatch),
		GetFlavorTypeCond(opts.FlavorTypes, opts.PartialMatch),
		GetFlavorCond(opts.Flavors, opts.PartialMatch),
		GetAbbreviationCond(opts.Abbreviations),
		GetContentFormatCond(opts.ContentFormats, opts.PartialMatch),
		GetBookCond(opts.Books),
		GetLanguageCond(opts.Languages, opts.PartialMatch),
		GetCheckingLevelCond(opts.CheckingLevels),
		GetMetadataTypeCond(opts.MetadataTypes, opts.PartialMatch),
		GetTagCond(opts.Tags),
		repoCond,
		ownerCond,
		stageCond,
		historyCond,
		langIsGLCond,
		keywordCond,
		builder.Eq{"`repository`.is_private": false},
		builder.Eq{"`repository`.is_archived": false})

	if len(opts.MetadataTypes) > 0 {
		cond.And(GetMetadataVersionCond(opts.MetadataVersions, opts.PartialMatch))
	}

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
		return builder.Lte{"`door43_metadata`.stage": StageOther}
	}
	return builder.Eq{"`door43_metadata`.is_latest_for_stage": true}
}

// GetSubjectCond gets the subject condition
func GetSubjectCond(subjects []string, partialMatch bool) builder.Cond {
	subjectCond := builder.NewCond()
	for _, subject := range subjects {
		for _, v := range strings.Split(subject, ",") {
			if partialMatch {
				subjectCond = subjectCond.Or(builder.Like{"`door43_metadata`.subject", strings.TrimSpace(v)})
			} else {
				subjectCond = subjectCond.Or(builder.Eq{"`door43_metadata`.subject": strings.TrimSpace(v)})
			}
		}
	}
	return subjectCond
}

// GetFlavorTypeCond gets the flavor type condition
func GetFlavorTypeCond(flavorTypes []string, partialMatch bool) builder.Cond {
	flavorTypeCond := builder.NewCond()
	for _, flavorType := range flavorTypes {
		for _, v := range strings.Split(flavorType, ",") {
			if partialMatch {
				flavorTypeCond = flavorTypeCond.Or(builder.Like{"`door43_metadata`.flavor_type", strings.TrimSpace(v)})
			} else {
				flavorTypeCond = flavorTypeCond.Or(builder.Eq{"`door43_metadata`.flavor_type": strings.TrimSpace(v)})
			}
		}
	}
	return flavorTypeCond
}

// GetFlavorCond gets the flavor type condition
func GetFlavorCond(flavors []string, partialMatch bool) builder.Cond {
	flavorCond := builder.NewCond()
	for _, flavor := range flavors {
		for _, v := range strings.Split(flavor, ",") {
			if partialMatch {
				flavorCond = flavorCond.Or(builder.Like{"`door43_metadata`.flavor", strings.TrimSpace(v)})
			} else {
				flavorCond = flavorCond.Or(builder.Eq{"`door43_metadata`.flavor": strings.TrimSpace(v)})
			}
		}
	}
	return flavorCond
}

// GetAbbreviationCond gets the abbreviation condition
func GetAbbreviationCond(abberviations []string) builder.Cond {
	abbreviationCond := builder.NewCond()
	for _, abbreviation := range abberviations {
		for _, v := range strings.Split(abbreviation, ",") {
			abbreviationCond = abbreviationCond.Or(builder.Eq{"`door43_metadata`.abbreviation": strings.TrimSpace(v)})
		}
	}
	return abbreviationCond
}

// GetContentFormatCond gets the metdata type condition
func GetContentFormatCond(formats []string, partialMatch bool) builder.Cond {
	formatCond := builder.NewCond()
	for _, format := range formats {
		for _, v := range strings.Split(format, ",") {
			if partialMatch {
				formatCond = formatCond.Or(builder.Like{"`door43_metadata`.content_format", strings.TrimSpace(v)})
			} else {
				formatCond = formatCond.Or(builder.Eq{"`door43_metadata`.content_format": strings.TrimSpace(v)})
			}
		}
	}
	return formatCond
}

// GetMetadataTypeCond gets the metdata type condition
func GetMetadataTypeCond(types []string, partialMatch bool) builder.Cond {
	metadataTypeCond := builder.NewCond()
	for _, metadataType := range types {
		for _, v := range strings.Split(metadataType, ",") {
			metadataTypeCond = metadataTypeCond.Or(builder.Eq{"`door43_metadata`.metadata_type": strings.ToLower(v)})
		}
	}
	return metadataTypeCond
}

// GetMetadataVersionCond gets the metdata version condition
func GetMetadataVersionCond(versions []string, partialMatch bool) builder.Cond {
	versionCond := builder.NewCond()
	for _, version := range versions {
		for _, v := range strings.Split(version, ",") {
			if partialMatch {
				versionCond = versionCond.Or(builder.Like{"`door43_metadata`.metadata_version", strings.TrimSpace(v)})
			} else {
				versionCond = versionCond.Or(builder.Eq{"`door43_metadata`.metadata_version": strings.TrimSpace(v)})
			}
		}
	}
	return versionCond
}

// GetLanguageCond gets the language condition
func GetLanguageCond(languages []string, partialMatch bool) builder.Cond {
	langCond := builder.NewCond()
	for _, lang := range languages {
		for _, v := range strings.Split(lang, ",") {
			if partialMatch {
				langCond = langCond.
					Or(builder.Like{"`door43_metadata`.language", strings.TrimSpace(v)}).
					Or(builder.Like{"CONCAT(SUBSTRING_INDEX(`repository`.lower_name, '_', 1), '_')", strings.TrimSpace(v) + "\\_"})
			} else {
				langCond = langCond.
					Or(builder.Eq{"`door43_metadata`.language": strings.TrimSpace(v)}).
					Or(builder.Eq{"CONCAT(SUBSTRING_INDEX(`repository`.lower_name, '_', 1), '_')": strings.TrimSpace(v) + "_"})
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
			bookCond = bookCond.Or(builder.Expr("JSON_CONTAINS(LOWER(JSON_EXTRACT(`door43_metadata`.ingredients, '$')), JSON_OBJECT('identifier', ?))", strings.ToLower(v)))
		}
	}
	return bookCond
}

// GetCheckingLevelCond gets the checking level condition
func GetCheckingLevelCond(checkingLevels []string) builder.Cond {
	checkingCond := builder.NewCond()
	for _, checking := range checkingLevels {
		for _, v := range strings.Split(checking, ",") {
			checkingCond = checkingCond.Or(builder.Gte{"`door43_metadata`.checking_level": v})
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
