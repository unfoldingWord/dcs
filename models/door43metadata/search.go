// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/optional"

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
	RepoID                   int64
	Keywords                 []string
	Owners                   []string
	Repos                    []string
	Tags                     []string
	Stage                    Stage
	Subjects                 []string
	FlavorTypes              []string
	Flavors                  []string
	Abbreviations            []string
	ContentFormats           []string
	CheckingLevels           []string
	Books                    []string
	IsRepoMetadata           bool
	IncludeHistory           bool
	MetadataTypes            []string
	MetadataVersions         []string
	Topics                   []string
	InvertedTopics           []string
	Healthchecks             []string
	IsHealthy                optional.Option[bool]
	IsHealthyWithoutWarnings optional.Option[bool]
	ShowIngredients          optional.Option[bool]
	Languages                []string
	LanguageIsGL             optional.Option[bool]
	HasAudio                 optional.Option[bool]
	HasVideo                 optional.Option[bool]
	HasPDF                   optional.Option[bool]
	HasStream                optional.Option[bool]
	HasOther                 optional.Option[bool]
	HasAttachment            optional.Option[bool]
	StartDateUnix            int64 // release_date_unix must be >= this when > 0
	EndDateUnix              int64 // release_date_unix must be <= this when > 0
	OrderBy                  []CatalogOrderBy
	PartialMatch             bool
}

// GetMetadataCond Get the metadata condition
func GetMetadataCond(keyword string) builder.Cond {
	cond := builder.NewCond()
	cond = cond.And(builder.Like{"`door43_metadata`.title", keyword})
	cond = cond.Or(builder.Eq{"`door43_metadata`.abbreviation": keyword})
	cond = cond.Or(builder.Like{"`door43_metadata`.subject", keyword})
	cond = cond.Or(builder.Expr("LOWER(`door43_metadata`.language) = ?", strings.ToLower(keyword)))
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
	if opts.LanguageIsGL.Has() {
		langIsGLCond = builder.Eq{"`door43_metadata`.language_is_gl": opts.LanguageIsGL.Value()}
	}

	isRepoMetadataCond := builder.NewCond()
	if opts.IsRepoMetadata {
		isRepoMetadataCond = builder.Eq{"`door43_metadata`.is_repo_metadata": true}
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
		GetTopicCond(opts.Topics, opts.PartialMatch),
		GetInvertedTopicCond(opts.InvertedTopics, opts.PartialMatch),
		GetHealthcheckCond(opts.Healthchecks),
		GetIsHealthyCond(opts.IsHealthy, opts.IsHealthyWithoutWarnings),
		GetContentFlagsCond(opts),
		GetReleaseDateCond(opts.StartDateUnix, opts.EndDateUnix),
		GetTagCond(opts.Tags),
		repoCond,
		ownerCond,
		stageCond,
		historyCond,
		langIsGLCond,
		keywordCond,
		isRepoMetadataCond,
		builder.Eq{"`repository`.is_private": false},
		builder.Eq{"`repository`.is_archived": false},
		builder.IsNull{"`door43_metadata`.validation_error"})

	if len(opts.MetadataTypes) > 0 {
		cond = cond.And(GetMetadataVersionCond(opts.MetadataVersions, opts.PartialMatch))
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
	if stage != StageNotSet {
		return builder.Lte{"`door43_metadata`.stage": stage}
	}
	return nil
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
		for v := range strings.SplitSeq(subject, ",") {
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
		for v := range strings.SplitSeq(flavorType, ",") {
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
		for v := range strings.SplitSeq(flavor, ",") {
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
		for v := range strings.SplitSeq(abbreviation, ",") {
			abbreviationCond = abbreviationCond.Or(builder.Eq{"`door43_metadata`.abbreviation": strings.TrimSpace(v)})
		}
	}
	return abbreviationCond
}

// GetContentFormatCond gets the metdata type condition
func GetContentFormatCond(formats []string, partialMatch bool) builder.Cond {
	formatCond := builder.NewCond()
	for _, format := range formats {
		for v := range strings.SplitSeq(format, ",") {
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
		for v := range strings.SplitSeq(metadataType, ",") {
			metadataTypeCond = metadataTypeCond.Or(builder.Eq{"`door43_metadata`.metadata_type": strings.ToLower(v)})
		}
	}
	return metadataTypeCond
}

// GetTopicCond gets the topic condition
func GetTopicCond(topics []string, partialMatch bool) builder.Cond {
	topicCond := builder.NewCond()
	for _, topic := range topics {
		for v := range strings.SplitSeq(topic, ",") {
			if partialMatch {
				topicCond = topicCond.Or(builder.In("`repository`.id", builder.Select("repo_id").From("repo_topic").InnerJoin("topic", "`repo_topic`.topic_id = `topic`.id").Where(builder.Like{"`topic`.name", strings.TrimSpace(v)})))
			} else {
				topicCond = topicCond.Or(builder.In("`repository`.id", builder.Select("repo_id").From("repo_topic").InnerJoin("topic", "`repo_topic`.topic_id = `topic`.id").Where(builder.Eq{"`topic`.name": strings.TrimSpace(v)})))
			}
		}
	}
	return topicCond
}

// GetInvertedTopicCond gets the inverted topic condition
func GetInvertedTopicCond(topics []string, partialMatch bool) builder.Cond {
	topicCond := builder.NewCond()
	for _, topic := range topics {
		for v := range strings.SplitSeq(topic, ",") {
			if partialMatch {
				topicCond = topicCond.And(builder.NotIn("`repository`.id", builder.Select("repo_id").From("repo_topic").InnerJoin("topic", "`repo_topic`.topic_id = `topic`.id").Where(builder.Like{"`topic`.name", strings.TrimSpace(v)})))
			} else {
				topicCond = topicCond.And(builder.NotIn("`repository`.id", builder.Select("repo_id").From("repo_topic").InnerJoin("topic", "`repo_topic`.topic_id = `topic`.id").Where(builder.Eq{"`topic`.name": strings.TrimSpace(v)})))
			}
		}
	}
	return topicCond
}

// SeverityLevel represents the level of severity or concern for a health check
type SeverityLevel int

const (
	SeverityLevelSuccess SeverityLevel = iota + 1 // 1
	SeverityLevelInfo                             // 2
	SeverityLevelWarning                          // 3
	SeverityLevelError                            // 4
)

// SeverityLevelMap map from string to SeverityLevel (int)
var SeverityLevelMap = map[string]SeverityLevel{
	"error":   SeverityLevelError,
	"warning": SeverityLevelWarning,
	"info":    SeverityLevelInfo,
	"success": SeverityLevelSuccess,
}

// GetHealthcheckCond gets the healthcheck condition
func GetHealthcheckCond(healthchecks []string) builder.Cond {
	healthcheckCond := builder.NewCond()
	for _, healthcheck := range healthchecks {
		for v := range strings.SplitSeq(healthcheck, ",") {
			v = strings.ToLower(strings.TrimSpace(v))
			if _, ok := SeverityLevelMap[v]; ok {
				healthcheckCond = healthcheckCond.Or(builder.Eq{"`door43_metadata`.healthcheck_severity": SeverityLevelMap[v]})
			}
		}
	}
	return healthcheckCond
}

// GetIsHealthyCond gets the condition for the is_healthy and is_healthy_without_warnings
// filters, both derived from healthcheck_severity. is_healthy ignores warnings;
// is_healthy_without_warnings does not. Entries never checked (NULL or 0) are not healthy.
func GetIsHealthyCond(isHealthy, isHealthyWithoutWarnings optional.Option[bool]) builder.Cond {
	cond := builder.NewCond()
	col := "`door43_metadata`.healthcheck_severity"
	notChecked := builder.IsNull{col}.Or(builder.Eq{col: 0})
	if isHealthy.Has() {
		if isHealthy.Value() {
			cond = cond.And(builder.In(col, SeverityLevelSuccess, SeverityLevelInfo, SeverityLevelWarning))
		} else {
			cond = cond.And(builder.Eq{col: SeverityLevelError}.Or(notChecked))
		}
	}
	if isHealthyWithoutWarnings.Has() {
		if isHealthyWithoutWarnings.Value() {
			cond = cond.And(builder.In(col, SeverityLevelSuccess, SeverityLevelInfo))
		} else {
			cond = cond.And(builder.In(col, SeverityLevelWarning, SeverityLevelError).Or(notChecked))
		}
	}
	return cond
}

// contentFlagColumns are the has_* release-attachment content flag columns, in a fixed
// order so generated SQL is deterministic.
var contentFlagColumns = []string{
	"`door43_metadata`.has_audio",
	"`door43_metadata`.has_video",
	"`door43_metadata`.has_pdf",
	"`door43_metadata`.has_stream",
	"`door43_metadata`.has_other",
}

// GetContentFlagsCond gets the condition for the has_* release-attachment content
// flags (HasAudio, HasVideo, HasPDF, HasStream, HasOther). HasAttachment matches
// entries where any (true) or none (false) of the five flags are set.
func GetContentFlagsCond(opts *SearchCatalogOptions) builder.Cond {
	cond := builder.NewCond()
	flagOpts := []optional.Option[bool]{opts.HasAudio, opts.HasVideo, opts.HasPDF, opts.HasStream, opts.HasOther}
	for i, opt := range flagOpts {
		if opt.Has() {
			cond = cond.And(builder.Eq{contentFlagColumns[i]: opt.Value()})
		}
	}
	if opts.HasAttachment.Has() {
		anyFlagCond := builder.NewCond()
		for _, col := range contentFlagColumns {
			anyFlagCond = anyFlagCond.Or(builder.Eq{col: true})
		}
		if opts.HasAttachment.Value() {
			cond = cond.And(anyFlagCond)
		} else {
			cond = cond.And(builder.Not{anyFlagCond})
		}
	}
	return cond
}

// GetReleaseDateCond bounds release_date_unix inclusively on both ends; a zero
// value disables that bound
func GetReleaseDateCond(startUnix, endUnix int64) builder.Cond {
	cond := builder.NewCond()
	if startUnix > 0 {
		cond = cond.And(builder.Gte{"`door43_metadata`.release_date_unix": startUnix})
	}
	if endUnix > 0 {
		cond = cond.And(builder.Lte{"`door43_metadata`.release_date_unix": endUnix})
	}
	return cond
}

// GetMetadataVersionCond gets the metdata version condition
func GetMetadataVersionCond(versions []string, partialMatch bool) builder.Cond {
	versionCond := builder.NewCond()
	for _, version := range versions {
		for v := range strings.SplitSeq(version, ",") {
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
		for v := range strings.SplitSeq(lang, ",") {
			lv := strings.ToLower(strings.TrimSpace(v)) // match case insensitively; lower_name is already lowercased
			if partialMatch {
				langCond = langCond.
					Or(builder.Expr("LOWER(`door43_metadata`.language) LIKE ?", "%"+lv+"%")).
					Or(builder.Expr("`repository`.lower_name LIKE ?", "%"+lv+"\\_%")) // %lang\_% — contains "lang_"
			} else {
				langCond = langCond.
					Or(builder.Expr("LOWER(`door43_metadata`.language) = ?", lv)).
					Or(builder.Expr("`repository`.lower_name LIKE ?", lv+"\\_%")) // lang\_% — starts with "lang_"
			}
		}
	}
	return langCond
}

// GetBookCond gets the book condition
func GetBookCond(books []string) builder.Cond {
	bookCond := builder.NewCond()
	for _, book := range books {
		for v := range strings.SplitSeq(book, ",") {
			bookCond = bookCond.Or(builder.Expr("JSON_SEARCH(dm.ingredients, 'one', ? COLLATE utf8mb4_general_ci, NULL, '$[*].identifier') IS NOT NULL", strings.ToLower(v)))
			// bookCond = bookCond.Or(builder.Expr("JSON_CONTAINS(LOWER(JSON_EXTRACT(dm.ingredients, '$')), JSON_OBJECT('identifier', ?))", strings.ToLower(v)))
		}
	}
	return bookCond
}

// GetCheckingLevelCond gets the checking level condition
func GetCheckingLevelCond(checkingLevels []string) builder.Cond {
	checkingCond := builder.NewCond()
	for _, checking := range checkingLevels {
		for v := range strings.SplitSeq(checking, ",") {
			checkingCond = checkingCond.Or(builder.Gte{"`door43_metadata`.checking_level": v})
		}
	}
	return checkingCond
}

// GetTagCond gets the tag condition
func GetTagCond(tags []string) builder.Cond {
	tagCond := builder.NewCond()
	for _, tag := range tags {
		for v := range strings.SplitSeq(tag, ",") {
			tagCond = tagCond.Or(builder.Eq{"`release`.tag_name": v})
		}
	}
	return tagCond
}

// GetRepoCond gets the repo condition
func GetRepoCond(repos []string, partialMatch bool) builder.Cond {
	repoCond := builder.NewCond()
	for _, repo := range repos {
		for v := range strings.SplitSeq(repo, ",") {
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
		for v := range strings.SplitSeq(owner, ",") {
			if partialMatch {
				ownerCond = ownerCond.Or(builder.Like{"`user`.lower_name", strings.ToLower(v)})
			} else {
				ownerCond = ownerCond.Or(builder.Eq{"`user`.lower_name": strings.ToLower(v)})
			}
		}
	}
	return ownerCond
}
