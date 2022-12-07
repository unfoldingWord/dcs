// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

/*** DCS Customizations - Router for Catalog page ***/

package dcs

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	// tplCatalog catalog page template.
	tplCatalog base.TplName = "catalog/catalog"
)

// CatalogSearchOptions when calling search catalog
type CatalogSearchOptions struct {
	PageSize int
	TplName  base.TplName
}

// RenderCatalogSearch render catalog search page
func RenderCatalogSearch(ctx *context.Context, opts *CatalogSearchOptions) {
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		dms     []*repo.Door43Metadata
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
	case "reverseridentifier":
		orderBy = door43metadata.CatalogOrderByIdentifierReverse
	case "identifier":
		orderBy = door43metadata.CatalogOrderByIdentifier
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

	var keywords, books, langs, subjects, repos, owners, tags, checkingLevels []string
	stage := door43metadata.StageProd
	query := strings.Trim(ctx.FormString("q"), " ")
	if query != "" {
		for _, token := range door43metadata.SplitAtCommaNotInString(query, true) {
			if strings.HasPrefix(token, "book:") {
				books = append(books, strings.TrimPrefix(token, "book:"))
			} else if strings.HasPrefix(token, "lang:") {
				langs = append(langs, strings.TrimPrefix(token, "lang:"))
			} else if strings.HasPrefix(token, "subject:") {
				subjects = append(subjects, strings.Trim(strings.TrimPrefix(token, "subject:"), `"`))
			} else if strings.HasPrefix(token, "repo:") {
				repos = append(repos, strings.TrimPrefix(token, "repo:"))
			} else if strings.HasPrefix(token, "owner:") {
				owners = append(owners, strings.TrimPrefix(token, "owner:"))
			} else if strings.HasPrefix(token, "tag:") {
				tags = append(tags, strings.TrimPrefix(token, "tag:"))
			} else if strings.HasPrefix(token, "checkinglevel:") {
				checkingLevels = append(checkingLevels, strings.TrimPrefix(token, "checkinglevel:"))
			} else if strings.HasPrefix(token, "stage:") {
				if s, ok := door43metadata.StageMap[strings.Trim(strings.TrimPrefix(token, "stage:"), `"`)]; ok {
					stage = s
				} else {
					stage = 0 // Makes it invalid, return no results
				}
			} else {
				keywords = append(keywords, token)
			}
		}
	}

	dms, count, err = models.SearchCatalog(&door43metadata.SearchCatalogOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: opts.PageSize,
		},
		OrderBy:         []door43metadata.CatalogOrderBy{orderBy},
		Keywords:        keywords,
		IncludeMetadata: true,
		Stage:           stage,
		IncludeHistory:  false,
		Books:           books,
		Subjects:        subjects,
		Languages:       langs,
		Repos:           repos,
		Owners:          owners,
		Tags:            tags,
		CheckingLevels:  checkingLevels,
	})
	if err != nil {
		ctx.ServerError("SearchCatalog", err)
		return
	}
	ctx.Data["Keyword"] = query
	ctx.Data["Total"] = count
	ctx.Data["Door43Metadatas"] = dms
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	pager := context.NewPagination(int(count), opts.PageSize, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "topic", "TopicOnly")
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
