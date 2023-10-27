// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
)

const (
	tplOrgHome base.TplName = "org/home"
)

// Home show organization home page
func Home(ctx *context.Context) {
	uname := ctx.Params(":username")

	if strings.HasSuffix(uname, ".keys") || strings.HasSuffix(uname, ".gpg") {
		ctx.NotFound("", nil)
		return
	}

	ctx.SetParams(":org", uname)
	context.HandleOrgAssignment(ctx)
	if ctx.Written() {
		return
	}

	org := ctx.Org.Organization

	ctx.Data["PageIsUserProfile"] = true
	ctx.Data["Title"] = org.DisplayName()
	if len(org.Description) != 0 {
		desc, err := markdown.RenderString(&markup.RenderContext{
			Ctx:       ctx,
			URLPrefix: ctx.Repo.RepoLink,
			Metas:     map[string]string{"mode": "document"},
			GitRepo:   ctx.Repo.GitRepo,
		}, org.Description)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
		ctx.Data["RenderedDescription"] = desc
	}

	var orderBy db.SearchOrderBy
	ctx.Data["SortType"] = ctx.FormString("sort")
	switch ctx.FormString("sort") {
	case "newest":
		orderBy = db.SearchOrderByNewest
	case "oldest":
		orderBy = db.SearchOrderByOldest
	case "recentupdate":
		orderBy = db.SearchOrderByRecentUpdated
	case "leastupdate":
		orderBy = db.SearchOrderByLeastUpdated
	case "reversealphabetically":
		orderBy = db.SearchOrderByAlphabeticallyReverse
	case "alphabetically":
		orderBy = db.SearchOrderByAlphabetically
	case "moststars":
		orderBy = db.SearchOrderByStarsReverse
	case "feweststars":
		orderBy = db.SearchOrderByStars
	case "mostforks":
		orderBy = db.SearchOrderByForksReverse
	case "fewestforks":
		orderBy = db.SearchOrderByForks
	default:
		ctx.Data["SortType"] = "recentupdate"
		orderBy = db.SearchOrderByRecentUpdated
	}

	keyword := ctx.FormTrim("q")
	ctx.Data["Keyword"] = keyword

	language := ctx.FormTrim("language")
	ctx.Data["Language"] = language

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	/*** DCS Customizations ***/
	var books, langs, keywords, subjects, resources, contentFormats, repoNames, owners, metadataTypes, metadataVersions []string
	if keyword != "" {
		for _, token := range door43metadata.SplitAtCommaNotInString(keyword, true) {
			if strings.HasPrefix(token, "book:") {
				books = append(books, strings.TrimPrefix(token, "book:"))
			} else if strings.HasPrefix(token, "lang:") {
				langs = append(langs, strings.TrimPrefix(token, "lang:"))
			} else if strings.HasPrefix(token, "subject:") {
				subjects = append(subjects, strings.Trim(strings.TrimPrefix(token, "subject:"), `"`))
			} else if strings.HasPrefix(token, "resource:") {
				resources = append(resources, strings.Trim(strings.TrimPrefix(token, "resource:"), `"`))
			} else if strings.HasPrefix(token, "format:") {
				contentFormats = append(contentFormats, strings.Trim(strings.TrimPrefix(token, "format:"), `"`))
			} else if strings.HasPrefix(token, "repo:") {
				repoNames = append(repoNames, strings.TrimPrefix(token, "repo:"))
			} else if strings.HasPrefix(token, "owner:") {
				owners = append(owners, strings.TrimPrefix(token, "owner:"))
			} else if strings.HasPrefix(token, "metadata_type:") {
				metadataTypes = append(metadataTypes, strings.TrimPrefix(token, "metadata_type:"))
			} else if strings.HasPrefix(token, "metadata_version:") {
				metadataVersions = append(metadataVersions, strings.TrimPrefix(token, "metadata_version:"))
			} else {
				keywords = append(keywords, token)
			}
		}
	}
	/*** END DCS Customizations ***/

	var (
		repos repo_model.RepositoryList // DCS Customizations - Fixed this
		count int64
		err   error
	)
	repos, count, err = repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		},
		Keyword:            strings.Join(keywords, ", "),
		OwnerID:            org.ID,
		OrderBy:            orderBy,
		Private:            ctx.IsSigned,
		Actor:              ctx.Doer,
		Language:           language,
		IncludeDescription: setting.UI.SearchRepoDescription,
		Books:              books,            // DCS Customizations
		Languages:          langs,            // DCS Customizations
		Subjects:           subjects,         // DCS Customizations
		Resources:          resources,        // DCS Customizations
		ContentFormats:     contentFormats,   // DCS Customizations
		Repos:              repoNames,        // DCS Customizations
		Owners:             owners,           // DCS Customizations
		MetadataTypes:      metadataTypes,    // DCS Customizations
		MetadataVersions:   metadataVersions, // DCS Customizations
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
	/*** End DCS Customizations ***/

	opts := &organization.FindOrgMembersOpts{
		OrgID:       org.ID,
		PublicOnly:  ctx.Org.PublicMemberOnly,
		ListOptions: db.ListOptions{Page: 1, PageSize: 25},
	}
	members, _, err := organization.FindOrgMembers(ctx, opts)
	if err != nil {
		ctx.ServerError("FindOrgMembers", err)
		return
	}

	var isFollowing bool
	if ctx.Doer != nil {
		isFollowing = user_model.IsFollowing(ctx, ctx.Doer.ID, ctx.ContextUser.ID)
	}

	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = count
	ctx.Data["Members"] = members
	ctx.Data["Teams"] = ctx.Org.Teams
	ctx.Data["DisableNewPullMirrors"] = setting.Mirror.DisableNewPull
	ctx.Data["PageIsViewRepositories"] = true
	ctx.Data["IsFollowing"] = isFollowing

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	pager := context.NewPagination(int(count), setting.UI.User.RepoPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "language", "Language")
	ctx.Data["Page"] = pager

	ctx.Data["ShowMemberAndTeamTab"] = ctx.Org.IsMember || len(members) > 0

	ctx.HTML(http.StatusOK, tplOrgHome)
}
