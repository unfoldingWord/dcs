// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

/*** DCS Customizations - Router for Catalog page ***/

package dcs

import (
	"strings"

	"gitea.dev/models"
	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	"gitea.dev/models/repo"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
)

const (
	// tplCatalog catalog page template.
	tplCatalog templates.TplName = "catalog/catalog"
)

// CatalogSearchOptions when calling search catalog
type CatalogSearchOptions struct {
	PageSize int
	TplName  templates.TplName
}

// RenderCatalogSearch render catalog search page
func RenderCatalogSearch(ctx *context.Context, opts *CatalogSearchOptions) {
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		dms     repo.Door43MetadataList
		count   int64
		err     error
		orderBy door43metadata.CatalogOrderBy
	)

	ctx.Data["SortType"] = ctx.FormString("sort")
	switch ctx.FormString("sort") {
	case "newest":
		orderBy = door43metadata.CatalogOrderByNewest
	case "oldest":
		orderBy = door43metadata.CatalogOrderByOldest
	case "reversetitle":
		orderBy = door43metadata.CatalogOrderByTitleReverse
	case "title":
		orderBy = door43metadata.CatalogOrderByTitle
	case "reversesubject":
		orderBy = door43metadata.CatalogOrderBySubjectReverse
	case "subject":
		orderBy = door43metadata.CatalogOrderBySubject
	case "reverseflavortype":
		orderBy = door43metadata.CatalogOrderByFlavorTypeReverse
	case "falvortype":
		orderBy = door43metadata.CatalogOrderByFlavorType
	case "reverseflavor":
		orderBy = door43metadata.CatalogOrderByFlavorReverse
	case "falvor":
		orderBy = door43metadata.CatalogOrderByFlavor
	case "reverserabbreviation":
		orderBy = door43metadata.CatalogOrderByAbbreviationReverse
	case "abbreviation":
		orderBy = door43metadata.CatalogOrderByAbbreviation
	case "reverserepo":
		orderBy = door43metadata.CatalogOrderByRepoNameReverse
	case "repo":
		orderBy = door43metadata.CatalogOrderByRepoName
	case "reversetag":
		orderBy = door43metadata.CatalogOrderByTagReverse
	case "tag":
		orderBy = door43metadata.CatalogOrderByTag
	case "reverselangcode":
		orderBy = door43metadata.CatalogOrderByLangCodeReverse
	case "langcode":
		orderBy = door43metadata.CatalogOrderByLangCode
	case "mostreleases":
		orderBy = door43metadata.CatalogOrderByReleasesReverse
	case "fewestreleases":
		orderBy = door43metadata.CatalogOrderByReleases
	case "moststars":
		orderBy = door43metadata.CatalogOrderByStarsReverse
	case "feweststars":
		orderBy = door43metadata.CatalogOrderByStars
	case "mostforks":
		orderBy = door43metadata.CatalogOrderByForksReverse
	case "fewestforks":
		orderBy = door43metadata.CatalogOrderByForks
	default:
		ctx.Data["SortType"] = "newest"
		orderBy = door43metadata.CatalogOrderByNewest
	}

	query := ctx.FormTrim("q")
	searchFields := []string{"keyword", "book", "lang", "subject", "flavor_type", "flavor", "abbreviation", "content_format", "repo", "owner", "tag", "checking_level", "metadata_type", "metadata_version", "topic", "without_topic", "stage", "has", "include_history"}
	searchMap := map[string][]string{}
	for _, field := range searchFields {
		searchMap[field] = []string{}
	}
	currentField := "keyword"
	if query != "" {
		for token := range strings.SplitSeq(query, ",") {
			token = strings.TrimSpace(token)
			value := token
			for key := range searchMap {
				if strings.HasPrefix(token, key+":") {
					currentField = key
					value = strings.TrimSpace(strings.TrimPrefix(token, key+":"))
					break
				}
			}
			searchMap[currentField] = append(searchMap[currentField], value)
		}
	}
	stage := door43metadata.StageProd
	if len(searchMap["stage"]) > 0 {
		if val, ok := door43metadata.StageMap[searchMap["stage"][0]]; ok {
			stage = val
		}
	}
	includeHistory := false
	if len(searchMap["include_history"]) > 0 {
		switch strings.ToLower(searchMap["include_history"][0]) {
		case "1", "true", "yes":
			includeHistory = true
		}
	}

	searchOpts := &door43metadata.SearchCatalogOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: opts.PageSize,
		},
		OrderBy:          []door43metadata.CatalogOrderBy{orderBy},
		Keywords:         searchMap["keyword"],
		Stage:            stage,
		IncludeHistory:   includeHistory,
		Books:            searchMap["book"],
		Subjects:         searchMap["subject"],
		FlavorTypes:      searchMap["flavor_type"],
		Flavors:          searchMap["flavor"],
		Abbreviations:    searchMap["abbreviation"],
		ContentFormats:   searchMap["content_format"],
		Languages:        searchMap["lang"],
		Repos:            searchMap["repo"],
		Owners:           searchMap["owner"],
		MetadataTypes:    searchMap["metadata_type"],
		MetadataVersions: searchMap["metadata_version"],
		Topics:           searchMap["topic"],
		InvertedTopics:   searchMap["without_topic"],
		Tags:             searchMap["tag"],
		CheckingLevels:   searchMap["checking_level"],
		PartialMatch:     true,
	}
	// The "has" field is a single dropdown in the Search Builder; multiple
	// values are ANDed, e.g. "has:audio, video" requires both.
	for _, v := range searchMap["has"] {
		switch strings.ToLower(v) {
		case "audio":
			searchOpts.HasAudio = optional.Some(true)
		case "video":
			searchOpts.HasVideo = optional.Some(true)
		case "pdf":
			searchOpts.HasPDF = optional.Some(true)
		case "stream":
			searchOpts.HasStream = optional.Some(true)
		case "other":
			searchOpts.HasOther = optional.Some(true)
		case "attachment", "attachments", "any":
			searchOpts.HasAttachment = optional.Some(true)
		}
	}

	dms, count, err = models.SearchCatalog(ctx, searchOpts)
	if err != nil {
		ctx.ServerError("SearchCatalog", err)
		return
	}
	ctx.Data["Keyword"] = query
	ctx.Data["Total"] = count
	ctx.Data["Door43Metadatas"] = dms
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	pager := context.NewPagination(count, opts.PageSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(200, opts.TplName)
}

// Catalog render catalog page
func Catalog(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("catalog")
	ctx.Data["PageIsCatalog"] = true
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	RenderCatalogSearch(ctx, &CatalogSearchOptions{
		PageSize: setting.UI.ExplorePagingNum,
		TplName:  tplCatalog,
	})
}
