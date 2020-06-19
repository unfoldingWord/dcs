// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

/*** DCS Customizations - Router for Catalog page ***/

package dcs

import (
	"bytes"
	"strings"

	"code.gitea.io/gitea/models"
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

var (
	nullByte = []byte{0x00}
)

func isKeywordValid(keyword string) bool {
	return !bytes.Contains([]byte(keyword), nullByte)
}

// RenderCatalogSearch render catalog search page
func RenderCatalogSearch(ctx *context.Context, opts *CatalogSearchOptions) {
	page := ctx.QueryInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		dms     []*models.Door43Metadata
		count   int64
		err     error
		orderBy models.CatalogOrderBy
	)

	ctx.Data["SortType"] = ctx.Query("sort")
	switch ctx.Query("sort") {
	case "newest":
		orderBy = models.CatalogOrderByNewest
	case "oldest":
		orderBy = models.CatalogOrderByOldest
	case "reversetitle":
		orderBy = models.CatalogOrderByTitleReverse
	case "title":
		orderBy = models.CatalogOrderByTitle
	case "reversesubject":
		orderBy = models.CatalogOrderBySubjectReverse
	case "subject":
		orderBy = models.CatalogOrderBySubject
	case "reverselangname":
		orderBy = models.CatalogOrderByLangNameReverse
	case "langname":
		orderBy = models.CatalogOrderByLangName
	case "reverselangcode":
		orderBy = models.CatalogOrderByLangCodeReverse
	case "langcode":
		orderBy = models.CatalogOrderByLangCode
	case "mostreleases":
		orderBy = models.CatalogOrderByReleasesReverse
	case "fewestreleases":
		orderBy = models.CatalogOrderByReleases
	case "moststars":
		orderBy = models.CatalogOrderByStarsReverse
	case "feweststars":
		orderBy = models.CatalogOrderByStars
	case "mostforks":
		orderBy = models.CatalogOrderByForksReverse
	case "fewestforks":
		orderBy = models.CatalogOrderByForks
	default:
		ctx.Data["SortType"] = "newest"
		orderBy = models.CatalogOrderByNewest
	}

	keyword := strings.Trim(ctx.Query("q"), " ")
	topicOnly := ctx.QueryBool("topic")
	ctx.Data["TopicOnly"] = topicOnly

	dms, count, err = models.SearchCatalog(&models.SearchCatalogOptions{
		ListOptions: models.ListOptions{
			Page:     page,
			PageSize: opts.PageSize,
		},
		OrderBy: orderBy,
		Keyword: keyword,
		TopicOnly: topicOnly,
		IncludeAllMetadata: true,
	})
	if err != nil {
		ctx.ServerError("SearchCatalog", err)
		return
	}
	ctx.Data["Keyword"] = keyword
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
