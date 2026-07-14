// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"net/http"
	"strings"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/sitemap"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
)

const (
	// tplExploreRepos explore repositories page template
	tplExploreRepos        templates.TplName = "explore/repos"
	relevantReposOnlyParam string            = "only_show_relevant"
)

// RepoSearchOptions when calling search repositories
type RepoSearchOptions struct {
	OwnerID          int64
	Private          bool
	Restricted       bool
	PageSize         int
	OnlyShowRelevant bool
	TplName          templates.TplName
}

// RenderRepoSearch render repositories search page
// This function is also used to render the Admin Repository Management page.
func RenderRepoSearch(ctx *context.Context, opts *RepoSearchOptions) {
	// Sitemap index for sitemap paths
	page := ctx.PathParamInt("idx")
	isSitemap := ctx.PathParam("idx") != ""
	if page <= 1 {
		page = ctx.FormInt("page")
	}

	if page <= 0 {
		page = 1
	}

	if isSitemap {
		opts.PageSize = setting.UI.SitemapPagingNum
	}

	var (
		repos   repo_model.RepositoryList // DCS Customizations - Fixed this
		count   int64
		err     error
		orderBy db.SearchOrderBy
	)

	sortOrder := ctx.FormString("sort")
	if sortOrder == "" {
		sortOrder = setting.UI.ExploreDefaultSort
	}

	if order, ok := repo_model.OrderByFlatMap[sortOrder]; ok {
		orderBy = order
	} else {
		sortOrder = "recentupdate"
		orderBy = db.SearchOrderByRecentUpdated
	}
	ctx.Data["SortType"] = sortOrder

	keyword := ctx.FormTrim("q")

	ctx.Data["OnlyShowRelevant"] = opts.OnlyShowRelevant

	topicOnly := ctx.FormBool("topic")
	ctx.Data["TopicOnly"] = topicOnly

	/*** DCS Customizations ***/
	origKeyword := keyword
	searchFields := []string{"keyword", "book", "lang", "subject", "flavor_type", "flavor", "abbreviation", "content_format", "repo", "owner", "tag", "checking_level", "metadata_type", "metadata_version", "topic", "without_topic", "healthcheck", "stage"}
	searchMap := map[string][]string{}
	for _, field := range searchFields {
		searchMap[field] = []string{}
	}
	currentField := "keyword"
	if keyword != "" {
		for token := range strings.SplitSeq(keyword, ",") {
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
		keyword = strings.Join(searchMap["keyword"], ", ")
	}
	/*** END DCS Customizations ***/

	language := ctx.FormTrim("language")
	ctx.Data["Language"] = language

	archived := ctx.FormOptionalBool("archived")
	ctx.Data["IsArchived"] = archived

	fork := ctx.FormOptionalBool("fork")
	ctx.Data["IsFork"] = fork

	mirror := ctx.FormOptionalBool("mirror")
	ctx.Data["IsMirror"] = mirror

	template := ctx.FormOptionalBool("template")
	ctx.Data["IsTemplate"] = template

	private := ctx.FormOptionalBool("private")
	ctx.Data["IsPrivate"] = private

	repos, count, err = repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: opts.PageSize,
		},
		Actor:              ctx.Doer,
		OrderBy:            orderBy,
		Private:            opts.Private,
		Keyword:            keyword,
		OwnerID:            opts.OwnerID,
		AllPublic:          true,
		AllLimited:         true,
		TopicOnly:          topicOnly,
		Language:           language,
		IncludeDescription: setting.UI.SearchRepoDescription,
		Books:              searchMap["book"],             // DCS Customizations
		Languages:          searchMap["lang"],             // DCS Customizations
		Subjects:           searchMap["subject"],          // DCS Customizations
		FlavorTypes:        searchMap["flavor_type"],      // DCS Customizations
		Flavors:            searchMap["flavor"],           // DCS Customizations
		Abbreviations:      searchMap["abbreviation"],     // DCS Customizations
		ContentFormats:     searchMap["content_format"],   // DCS Customizations
		Repos:              searchMap["repo"],             // DCS Customizations
		Owners:             searchMap["owner"],            // DCS Customizations
		MetadataTypes:      searchMap["metadata_type"],    // DCS Customizations
		MetadataVersions:   searchMap["metadata_version"], // DCS Customizations
		Topics:             searchMap["topic"],            // DCS Customizations
		InvertedTopics:     searchMap["without_topic"],    // DCS Customizations
		Healthchecks:       searchMap["healthcheck"],      // DCS Customizations
		OnlyShowRelevant:   opts.OnlyShowRelevant,
		Archived:           archived,
		Fork:               fork,
		Mirror:             mirror,
		Template:           template,
		IsPrivate:          private,
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}

	/*** DCS Customizations ***/
	err = repos.LoadLatestDMs(ctx)
	if err != nil {
		log.Error("LoadLatestDMs: unable to load DMs for repos")
	}
	/*** END DCS Customizations ***/

	if isSitemap {
		m := sitemap.NewSitemap()
		for _, item := range repos {
			m.Add(sitemap.URL{URL: item.HTMLURL(), LastMod: item.UpdatedUnix.AsTimePtr()})
		}
		ctx.Resp.Header().Set("Content-Type", "text/xml")
		if _, err := m.WriteTo(ctx.Resp); err != nil {
			log.Error("Failed writing sitemap: %v", err)
		}
		return
	}

	ctx.Data["Keyword"] = origKeyword // DCS Customizations
	ctx.Data["Total"] = count
	ctx.Data["Repos"] = repos
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	pager := context.NewPagination(count, opts.PageSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, opts.TplName)
}

// Repos render explore repositories page
func Repos(ctx *context.Context) {
	ctx.Data["UsersPageIsDisabled"] = setting.Service.Explore.DisableUsersPage
	ctx.Data["OrganizationsPageIsDisabled"] = setting.Service.Explore.DisableOrganizationsPage
	ctx.Data["CodePageIsDisabled"] = setting.Service.Explore.DisableCodePage
	ctx.Data["Title"] = ctx.Tr("explore_title")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["ShowRepoOwnerOnList"] = true
	ctx.Data["PageIsExploreRepositories"] = true
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	var ownerID int64
	if ctx.Doer != nil && !ctx.Doer.IsAdmin {
		ownerID = ctx.Doer.ID
	}

	onlyShowRelevant := setting.UI.OnlyShowRelevantRepos

	_ = ctx.Req.ParseForm() // parse the form first, to prepare the ctx.Req.Form field
	if len(ctx.Req.Form[relevantReposOnlyParam]) != 0 {
		onlyShowRelevant = ctx.FormBool(relevantReposOnlyParam)
	}

	RenderRepoSearch(ctx, &RepoSearchOptions{
		PageSize:         setting.UI.ExplorePagingNum,
		OwnerID:          ownerID,
		Private:          ctx.Doer != nil,
		TplName:          tplExploreRepos,
		OnlyShowRelevant: onlyShowRelevant,
	})
}
