// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package explore

import (
	"net/http"
	"strings" // DCS Customizations

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	// tplExploreRepos explore repositories page template
	tplExploreRepos base.TplName = "explore/repos"
)

// RepoSearchOptions when calling search repositories
type RepoSearchOptions struct {
	OwnerID    int64
	Private    bool
	Restricted bool
	PageSize   int
	TplName    base.TplName
}

// RenderRepoSearch render repositories search page
func RenderRepoSearch(ctx *context.Context, opts *RepoSearchOptions) {
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		repos   []*models.Repository
		count   int64
		err     error
		orderBy models.SearchOrderBy
	)

	ctx.Data["SortType"] = ctx.FormString("sort")
	switch ctx.FormString("sort") {
	case "newest":
		orderBy = models.SearchOrderByNewest
	case "oldest":
		orderBy = models.SearchOrderByOldest
	case "recentupdate":
		orderBy = models.SearchOrderByRecentUpdated
	case "leastupdate":
		orderBy = models.SearchOrderByLeastUpdated
	case "reversealphabetically":
		orderBy = models.SearchOrderByAlphabeticallyReverse
	case "alphabetically":
		orderBy = models.SearchOrderByAlphabetically
	case "reversesize":
		orderBy = models.SearchOrderBySizeReverse
	case "size":
		orderBy = models.SearchOrderBySize
	case "moststars":
		orderBy = models.SearchOrderByStarsReverse
	case "feweststars":
		orderBy = models.SearchOrderByStars
	case "mostforks":
		orderBy = models.SearchOrderByForksReverse
	case "fewestforks":
		orderBy = models.SearchOrderByForks
	default:
		ctx.Data["SortType"] = "recentupdate"
		orderBy = models.SearchOrderByRecentUpdated
	}

	keyword := ctx.FormTrim("q")
	topicOnly := ctx.FormBool("topic")
	ctx.Data["TopicOnly"] = topicOnly

	/*** DCS Customizations ***/
	var books, langs, keywords, subjects, repoNames, owners []string
	var origKeyword = keyword
	if keyword != "" {
		for _, token := range models.SplitAtCommaNotInString(keyword, true) {
			if strings.HasPrefix(token, "book:") {
				books = append(books, strings.TrimPrefix(token, "book:"))
			} else if strings.HasPrefix(token, "lang:") {
				langs = append(langs, strings.TrimPrefix(token, "lang:"))
			} else if strings.HasPrefix(token, "subject:") {
				subjects = append(subjects, strings.Trim(strings.TrimPrefix(token, "subject:"), `"`))
			} else if strings.HasPrefix(token, "repo:") {
				repoNames = append(repoNames, strings.TrimPrefix(token, "repo:"))
			} else if strings.HasPrefix(token, "owner:") {
				owners = append(owners, strings.TrimPrefix(token, "owner:"))
			} else {
				keywords = append(keywords, token)
			}
		}
		keyword = strings.Join(keywords, ", ")
	}
	/*** END DCS Customizations ***/

	repos, count, err = models.SearchRepository(&models.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: opts.PageSize,
		},
		Actor:              ctx.User,
		OrderBy:            orderBy,
		Private:            opts.Private,
		Keyword:            keyword,
		OwnerID:            opts.OwnerID,
		AllPublic:          true,
		AllLimited:         true,
		TopicOnly:          topicOnly,
		IncludeDescription: setting.UI.SearchRepoDescription,
		/*** DCS Customizaitons ***/
		Books:           books,
		Languages:       langs,
		Subjects:        subjects,
		Repos:           repoNames,
		Owners:          owners,
		IncludeMetadata: true,
		/*** END DCS Customations ***/
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}
	ctx.Data["Keyword"] = origKeyword // DCS Customizations
	ctx.Data["Total"] = count
	ctx.Data["Repos"] = repos
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	pager := context.NewPagination(int(count), opts.PageSize, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "topic", "TopicOnly")
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, opts.TplName)
}

// Repos render explore repositories page
func Repos(ctx *context.Context) {
	ctx.Data["UsersIsDisabled"] = setting.Service.Explore.DisableUsersPage
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExploreRepositories"] = true
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	var ownerID int64
	if ctx.User != nil && !ctx.User.IsAdmin {
		ownerID = ctx.User.ID
	}

	RenderRepoSearch(ctx, &RepoSearchOptions{
		PageSize: setting.UI.ExplorePagingNum,
		OwnerID:  ownerID,
		Private:  ctx.User != nil,
		TplName:  tplExploreRepos,
	})
}
