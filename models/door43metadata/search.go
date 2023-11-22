// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
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
	CatalogOrderByTitle              CatalogOrderBy = "`door43_metadata`.title ASC"
	CatalogOrderByTitleReverse       CatalogOrderBy = "`door43_metadata`.title DESC"
	CatalogOrderBySubject            CatalogOrderBy = "`door43_metadata`.subject ASC"
	CatalogOrderBySubjectReverse     CatalogOrderBy = "`door43_metadata`.subject DESC"
	CatalogOrderByResource           CatalogOrderBy = "`door43_metadata`.resource ASC"
	CatalogOrderByResourceReverse    CatalogOrderBy = "`door43_metadata`.resource DESC"
	CatalogOrderByRepoName           CatalogOrderBy = "`repository`.lower_name ASC"
	CatalogOrderByRepoNameReverse    CatalogOrderBy = "`repository`.lower_name DESC"
	CatalogOrderByTag                CatalogOrderBy = "`door43_metadata`.ref ASC"
	CatalogOrderByTagReverse         CatalogOrderBy = "`door43_metadata`.ref DESC"
	CatalogOrderByReleaseDate        CatalogOrderBy = "`door43_metadata`.ref ASC"
	CatalogOrderByReleaseDateReverse CatalogOrderBy = "`door43_metadata`.ref DESC"
	CatalogOrderByLangCode           CatalogOrderBy = "`door43_metadata`.language ASC"
	CatalogOrderByLangCodeReverse    CatalogOrderBy = "`door43_metadata`.language DESC"
	CatalogOrderByLangTitle          CatalogOrderBy = "`door43_metadata`.language_title ASC"
	CatalogOrderByLangTitleReverse   CatalogOrderBy = "`door43_metadata`.language_title DESC"
	CatalogOrderByOldest             CatalogOrderBy = "`door43_metadata`.release_date_unix ASC"
	CatalogOrderByNewest             CatalogOrderBy = "`door43_metadata`.release_date_unix DESC"
	CatalogOrderByReleases           CatalogOrderBy = "release_count ASC"
	CatalogOrderByReleasesReverse    CatalogOrderBy = "release_count DESC"
	CatalogOrderByStars              CatalogOrderBy = "`repository`.num_stars ASC"
	CatalogOrderByStarsReverse       CatalogOrderBy = "`repository`.num_stars DESC"
	CatalogOrderByForks              CatalogOrderBy = "`repository`.num_forks ASC"
	CatalogOrderByForksReverse       CatalogOrderBy = "`repository`.num_forks DESC"
	CatalogOrderByOwner              CatalogOrderBy = "`user`.name ASC"
	CatalogOrderByOwnerReverse       CatalogOrderBy = "`user`.name DESC"
)

type CatalogGroupBy string

func (s CatalogGroupBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	CatalogGroupByOwner         CatalogGroupBy = "`user`.name"
	CatalogGroupBySubject       CatalogGroupBy = "`door43_metadata`.subject"
	CatalogGroupByLanguage      CatalogGroupBy = "`door43_metadata`.language"
	CatalogGroupByLanguageTitle CatalogGroupBy = "`door43_metadata`.language_title"
	CatalogGroupByMetadataType  CatalogGroupBy = "`door43_metadata`.metadata_type"
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
	Resources        []string
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
	GroupBy          CatalogGroupBy
	PartialMatch     bool
}

var searchOrderByMap = map[string]map[string]CatalogOrderBy{
	"asc": {
		"title":    CatalogOrderByTitle,
		"subject":  CatalogOrderBySubject,
		"resource": CatalogOrderByResource,
		"reponame": CatalogOrderByRepoName,
		"released": CatalogOrderByOldest,
		"lang":     CatalogOrderByLangCode,
		"releases": CatalogOrderByReleases,
		"stars":    CatalogOrderByStars,
		"forks":    CatalogOrderByForks,
		"tag":      CatalogOrderByTag,
	},
	"desc": {
		"title":    CatalogOrderByTitleReverse,
		"subject":  CatalogOrderBySubjectReverse,
		"resouce":  CatalogOrderByResourceReverse,
		"reponame": CatalogOrderByRepoNameReverse,
		"released": CatalogOrderByNewest,
		"lang":     CatalogOrderByLangCodeReverse,
		"releases": CatalogOrderByReleasesReverse,
		"stars":    CatalogOrderByStarsReverse,
		"forks":    CatalogOrderByForksReverse,
		"tag":      CatalogOrderByTagReverse,
	},
}

// GetMetadataCond Get the metadata condition
func GetMetadataCond(keyword string) builder.Cond {
	cond := builder.NewCond()
	cond = cond.And(builder.Like{"`door43_metadata`.title", keyword})
	cond = cond.Or(builder.Eq{"`door43_metadata`.resource": keyword})
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
		GetResourceCond(opts.Resources),
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

// GetStageCond gets the condition for the given stage
func GetStageCond(stage Stage) builder.Cond {
	return builder.Lte{"`door43_metadata`.stage": stage}
}

// GetHistoryCond gets the conditions if IncludeHistory is false
func GetHistoryCond(includeHistory bool) builder.Cond {
	if includeHistory {
		return builder.Lte{"`door43_metadata`.stage": StageBranch}
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

// GetResourceCond gets the metdata type condition
func GetResourceCond(resources []string) builder.Cond {
	resourceCond := builder.NewCond()
	for _, resource := range resources {
		for _, v := range strings.Split(resource, ",") {
			resourceCond = resourceCond.Or(builder.Eq{"`door43_metadata`.resource": strings.TrimSpace(v)})
		}
	}
	return resourceCond
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

// GetSingleDMFieldList gets the values of a single field
func GetSingleDMFieldList(ctx *context.Context, field string) ([]string, error) {
	stageStr := ctx.FormString("stage")
	stage := StageProd
	if stageStr != "" {
		var ok bool
		stage, ok = StageMap[stageStr]
		if !ok {
			err := fmt.Errorf("invalid stage [%s]", stageStr)
			return nil, err
		}
	}

	metadataTypes := ctx.QueryStrings("metadataType")
	metadataVersions := ctx.QueryStrings("metadataVersion")

	listOptions := db.ListOptions{
		ListAll: true,
	}

	opts := &SearchCatalogOptions{
		ListOptions:      listOptions,
		Owners:           ctx.QueryStrings("owner"),
		Repos:            ctx.QueryStrings("repos"),
		Tags:             ctx.QueryStrings("tag"),
		Stage:            stage,
		Languages:        ctx.QueryStrings("lang"),
		LanguageIsGL:     ctx.FormOptionalBool("is_gl"),
		Subjects:         ctx.QueryStrings("subject"),
		Resources:        ctx.QueryStrings("resource"),
		ContentFormats:   ctx.QueryStrings("format"),
		CheckingLevels:   ctx.QueryStrings("checkingLevel"),
		Books:            ctx.QueryStrings("book"),
		IncludeHistory:   ctx.FormBool("includeHistory"),
		ShowIngredients:  ctx.FormOptionalBool("showIngredients"),
		MetadataTypes:    metadataTypes,
		MetadataVersions: metadataVersions,
		PartialMatch:     ctx.FormBool("partialMatch"),
	}

	sortModes := ctx.QueryStrings("sort")
	if len(sortModes) > 0 {
		sortOrder := ctx.FormString("order")
		if sortOrder == "" {
			sortOrder = "asc"
		}
		if searchModeMap, ok := searchOrderByMap[sortOrder]; ok {
			for _, sortMode := range sortModes {
				if orderBy, ok := searchModeMap[strings.ToLower(sortMode)]; ok {
					opts.OrderBy = append(opts.OrderBy, orderBy)
				} else {
					err := fmt.Errorf("invalid sort mode [%s]", sortMode)
					ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
						"ok":    false,
						"error": err.Error(),
					})
					return nil, err
				}
			}
		} else {
			err := fmt.Errorf("invalid sort order [%s]", sortOrder)
			return nil, err
		}
	} else {
		opts.OrderBy = []CatalogOrderBy{CatalogOrderByLangCode, CatalogOrderBySubject, CatalogOrderByReleaseDateReverse}
	}

	results, err := SearchDoor43MetadataField(ctx, opts, field)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// SearchCatalog returns catalog repositories based on search options,
// it returns results in given range and number of total results.
func SearchCatalog(ctx context.Context, opts *SearchCatalogOptions) (repo.Door43MetadataList, int64, error) {
	cond := SearchCatalogCondition(opts)
	return SearchCatalogByCondition(ctx, opts, cond)
}

// SearchCatalogByCondition search repositories by condition
func SearchCatalogByCondition(ctx context.Context, opts *SearchCatalogOptions, cond builder.Cond) (repo.Door43MetadataList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize < 0 {
		opts.PageSize = 0
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = []CatalogOrderBy{CatalogOrderByNewest}
	}

	var dms repo.Door43MetadataList
	if opts.PageSize > 0 {
		dms = make(repo.Door43MetadataList, 0, opts.PageSize)
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
		return nil, 0, err
	}

	releaseInfoJoinCondition := "release_info.repo_id = `door43_metadata`.repo_id"
	if !opts.IncludeHistory {
		releaseInfoJoinCondition += " AND release_info.latest_unix = `door43_metadata`.release_date_unix AND release_info.latest_stage = `door43_metadata`.stage"
	}

	sess := db.GetEngine(db.DefaultContext).
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Join("INNER", "user", "`repository`.owner_id = `user`.id").
		Join("LEFT", "release", "`release`.id = `door43_metadata`.release_id").
		Join("INNER", "("+releaseInfoOuter+") release_info", releaseInfoJoinCondition).
		Where(cond)

	for _, orderBy := range opts.OrderBy {
		sess.OrderBy(orderBy.String())
	}

	if opts.GroupBy != "" {
		sess.GroupBy(opts.GroupBy.String())
	}

	if opts.PageSize > 0 || opts.Page > 1 {
		sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	count, err := sess.FindAndCount(&dms)
	if err != nil {
		return nil, 0, fmt.Errorf("FindAndCount: %v", err)
	}

	if opts.GroupBy == "" {
		if err = dms.LoadAttributes(ctx); err != nil {
			return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
		}
	}

	return dms, count, nil
}

// SearchDoor43MetadataField returns door43metadat field based on search options
func SearchDoor43MetadataField(ctx context.Context, opts *SearchCatalogOptions, field string) ([]string, error) {
	cond := SearchCatalogCondition(opts)
	return SearchDoor43MetadataFieldByCondition(ctx, opts, cond, field)
}

// SearchDoor43MetadataFieldByCondition search door43metadata entries by condition for a single field
func SearchDoor43MetadataFieldByCondition(ctx context.Context, opts *SearchCatalogOptions, cond builder.Cond, field string) ([]string, error) {
	var results []string

	if !strings.Contains(field, ".") {
		field = "`door43_metadata`." + field
	}

	sess := db.GetEngine(db.DefaultContext).Table("door43_metadata").
		Select("DISTINCT "+field).
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Join("INNER", "user", "`repository`.owner_id = `user`.id").
		Where(cond).
		OrderBy(field)

	err := sess.Find(&results)
	if err != nil {
		return nil, fmt.Errorf("find: %v", err)
	}

	return results, nil
}
