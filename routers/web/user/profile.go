// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"net/http"
	"strings"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/feed"
	"code.gitea.io/gitea/routers/web/org"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
)

// OwnerProfile render profile page for a user or a organization (aka, repo owner)
func OwnerProfile(ctx *context.Context) {
	if strings.Contains(ctx.Req.Header.Get("Accept"), "application/rss+xml") {
		feed.ShowUserFeedRSS(ctx)
		return
	}
	if strings.Contains(ctx.Req.Header.Get("Accept"), "application/atom+xml") {
		feed.ShowUserFeedAtom(ctx)
		return
	}

	if ctx.ContextUser.IsOrganization() {
		org.Home(ctx)
	} else {
		userProfile(ctx)
	}
}

func userProfile(ctx *context.Context) {
	// check view permissions
	if !user_model.IsUserVisibleToViewer(ctx, ctx.ContextUser, ctx.Doer) {
		ctx.NotFound("user", fmt.Errorf(ctx.ContextUser.Name))
		return
	}

	ctx.Data["Title"] = ctx.ContextUser.DisplayName()
	ctx.Data["PageIsUserProfile"] = true

	// prepare heatmap data
	if setting.Service.EnableUserHeatmap {
		data, err := activities_model.GetUserHeatmapDataByUser(ctx, ctx.ContextUser, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserHeatmapDataByUser", err)
			return
		}
		ctx.Data["HeatmapData"] = data
		ctx.Data["HeatmapTotalContributions"] = activities_model.GetTotalContributionsInHeatmap(data)
	}

	profileGitRepo, profileReadmeBlob, profileClose := shared_user.FindUserProfileReadme(ctx)
	defer profileClose()

	showPrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == ctx.ContextUser.ID)
	prepareUserProfileTabData(ctx, showPrivate, profileGitRepo, profileReadmeBlob)
	// call PrepareContextForProfileBigAvatar later to avoid re-querying the NumFollowers & NumFollowing
	shared_user.PrepareContextForProfileBigAvatar(ctx)
	ctx.HTML(http.StatusOK, tplProfile)
}

func prepareUserProfileTabData(ctx *context.Context, showPrivate bool, profileGitRepo *git.Repository, profileReadme *git.Blob) {
	// if there is a profile readme, default to "overview" page, otherwise, default to "repositories" page
	// if there is not a profile readme, the overview tab should be treated as the repositories tab
	tab := ctx.FormString("tab")
	if tab == "" || tab == "overview" {
		if profileReadme != nil {
			tab = "overview"
		} else {
			tab = "repositories"
		}
	}
	ctx.Data["TabName"] = tab
	ctx.Data["HasProfileReadme"] = profileReadme != nil

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	pagingNum := setting.UI.User.RepoPagingNum
	topicOnly := ctx.FormBool("topic")
	var (
		repos   []*repo_model.Repository
		count   int64
		total   int
		orderBy db.SearchOrderBy
	)

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

	followers, numFollowers, err := user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{
		PageSize: pagingNum,
		Page:     page,
	})
	if err != nil {
		ctx.ServerError("GetUserFollowers", err)
		return
	}
	ctx.Data["NumFollowers"] = numFollowers
	following, numFollowing, err := user_model.GetUserFollowing(ctx, ctx.ContextUser, ctx.Doer, db.ListOptions{
		PageSize: pagingNum,
		Page:     page,
	})
	if err != nil {
		ctx.ServerError("GetUserFollowing", err)
		return
	}
	ctx.Data["NumFollowing"] = numFollowing

	switch tab {
	case "followers":
		ctx.Data["Cards"] = followers
		total = int(numFollowers)
	case "following":
		ctx.Data["Cards"] = following
		total = int(numFollowing)
	case "activity":
		date := ctx.FormString("date")
		pagingNum = setting.UI.FeedPagingNum
		items, count, err := activities_model.GetFeeds(ctx, activities_model.GetFeedsOptions{
			RequestedUser:   ctx.ContextUser,
			Actor:           ctx.Doer,
			IncludePrivate:  showPrivate,
			OnlyPerformedBy: true,
			IncludeDeleted:  false,
			Date:            date,
			ListOptions: db.ListOptions{
				PageSize: pagingNum,
				Page:     page,
			},
		})
		if err != nil {
			ctx.ServerError("GetFeeds", err)
			return
		}
		ctx.Data["Feeds"] = items
		ctx.Data["Date"] = date

		total = int(count)
	case "stars":
		ctx.Data["PageIsProfileStarList"] = true
		repos, count, err = repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: pagingNum,
				Page:     page,
			},
			Actor:              ctx.Doer,
			Keyword:            keyword,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			StarredByID:        ctx.ContextUser.ID,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			Language:           language,
			IncludeDescription: setting.UI.SearchRepoDescription,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	case "watching":
		repos, count, err = repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: pagingNum,
				Page:     page,
			},
			Actor:              ctx.Doer,
			Keyword:            keyword,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			WatchedByID:        ctx.ContextUser.ID,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			Language:           language,
			IncludeDescription: setting.UI.SearchRepoDescription,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	case "overview":
		if bytes, err := profileReadme.GetBlobContent(setting.UI.MaxDisplayFileSize); err != nil {
			log.Error("failed to GetBlobContent: %v", err)
		} else {
			if profileContent, err := markdown.RenderString(&markup.RenderContext{
				Ctx:     ctx,
				GitRepo: profileGitRepo,
				Metas:   map[string]string{"mode": "document"},
			}, bytes); err != nil {
				log.Error("failed to RenderString: %v", err)
			} else {
				ctx.Data["ProfileReadme"] = profileContent
			}
		}
	default: // default to "repositories"
		/*** DCS Customizations ***/
		var books, langs, keywords, subjects, flavorTypes, flavors, abbreviations, contentFormats, repoNames, owners, metadataTypes, metadataVersions []string
		if keyword != "" {
			for _, token := range door43metadata.SplitAtCommaNotInString(keyword, true) {
				if strings.HasPrefix(token, "book:") {
					books = append(books, strings.TrimPrefix(token, "book:"))
				} else if strings.HasPrefix(token, "lang:") {
					langs = append(langs, strings.TrimPrefix(token, "lang:"))
				} else if strings.HasPrefix(token, "subject:") {
					subjects = append(subjects, strings.Trim(strings.TrimPrefix(token, "subject:"), `"`))
				} else if strings.HasPrefix(token, "flavor_type:") {
					flavorTypes = append(flavorTypes, strings.Trim(strings.TrimPrefix(token, "flavor_type:"), `"`))
				} else if strings.HasPrefix(token, "flavor:") {
					flavors = append(flavors, strings.Trim(strings.TrimPrefix(token, "flavor:"), `"`))
				} else if strings.HasPrefix(token, "abbreviation:") {
					abbreviations = append(abbreviations, strings.Trim(strings.TrimPrefix(token, "abbreviation:"), `"`))
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

		repos, count, err = repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: pagingNum,
				Page:     page,
			},
			Actor:              ctx.Doer,
			Keyword:            strings.Join(keywords, ", "), // DCS Customizations
			OwnerID:            ctx.ContextUser.ID,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			Language:           language,
			IncludeDescription: setting.UI.SearchRepoDescription,
			Books:              books,            // DCS Customizations
			Languages:          langs,            // DCS Customizations
			Subjects:           subjects,         // DCS Customizations
			FlavorTypes:        flavorTypes,      // DCS Customizations
			Flavors:            flavors,          // DCS Customization
			Abbreviations:      abbreviations,    // DCS Customizations
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

		total = int(count)
	}
	/*** DCS Customizations ***/
	for _, repo := range repos {
		if err := repo.LoadLatestDMs(ctx); err != nil {
			log.Error("Error LoadLatestDMs [%s]: %v", repo.FullName(), err)
		}
	}
	/*** End DCS Customizations ***/

	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = total

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	pager := context.NewPagination(total, pagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParam(ctx, "tab", "TabName")
	if tab != "followers" && tab != "following" && tab != "activity" && tab != "projects" {
		pager.AddParam(ctx, "language", "Language")
	}
	if tab == "activity" {
		pager.AddParam(ctx, "date", "Date")
	}
	ctx.Data["Page"] = pager
}

// Action response for follow/unfollow user request
func Action(ctx *context.Context) {
	var err error
	switch ctx.FormString("action") {
	case "follow":
		err = user_model.FollowUser(ctx, ctx.Doer.ID, ctx.ContextUser.ID)
	case "unfollow":
		err = user_model.UnfollowUser(ctx, ctx.Doer.ID, ctx.ContextUser.ID)
	}

	if err != nil {
		log.Error("Failed to apply action %q: %v", ctx.FormString("action"), err)
		ctx.JSONError(fmt.Sprintf("Action %q failed", ctx.FormString("action")))
		return
	}
	ctx.JSONOK()
}
