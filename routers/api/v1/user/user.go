// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	activities_model "code.gitea.io/gitea/models/activities"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
)

// Search search users
func Search(ctx *context.APIContext) {
	// swagger:operation GET /users/search user userSearch
	// ---
	// summary: Search for users
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: keyword
	//   type: string
	// - name: uid
	//   in: query
	//   description: ID of the user to search for
	//   type: integer
	//   format: int64
	// - name: lang
	//   in: query
	//   description: if the user has one or more repos with the given language(s), the user will be in the results. Multiple lang's are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: is_gl
	//   in: query
	//   description: if the user has one or more repos that is a gateway language, the user will be in the results
	//   type: boolean
	// - name: subject
	//   in: query
	//   description: if the user has one or more repos with the given subject(s), the user will be in the results. Multiple subjects are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [Aligned Bible,Aramaic Grammar,Bible,Greek Grammar,Greek Lexicon,Greek New Testament,Hebrew Grammar,Hebrew Old Testament,Hebrew-Aramaic Lexicon,OBS Study Notes,OBS Study Questions,OBS Translation Notes,OBS Translation Questions,Open Bible Stories,Study Notes,Study Questions,Training Library,Translation Academy,Translation Notes,Translation Questions,Translation Words,TSV Study Notes,TSV Study Questions,TSV Translation Notes,TSV Translation Questions,TSV Translation Words Links,TSV OBS Study Notes,TSV OBS Study Questions,TSV OBS Translation Notes,TSV OBS Translation Questions,TSV OBS Translation Words Links]
	// - name: metadataType
	//   in: query
	//   description: if the user has one or more repos with the given metadata type(s), the user will be in the results. Multiple metadata types are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [rc,sb,tc,ts]
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     description: "SearchResults of a successful search"
	//     schema:
	//       type: object
	//       properties:
	//         ok:
	//           type: boolean
	//         data:
	//           type: array
	//           items:
	//             "$ref": "#/definitions/User"

	listOptions := utils.GetListOptions(ctx)

	users, maxResults, err := user_model.SearchUsers(&user_model.SearchUserOptions{
		Actor:       ctx.Doer,
		Keyword:     ctx.FormTrim("q"),
		UID:         ctx.FormInt64("uid"),
		Type:        user_model.UserTypeIndividual,
		ListOptions: listOptions,
		// DCS Customizations
		RepoLanguages:     ctx.FormStrings("lang"),
		RepoSubjects:      ctx.FormStrings("subject"),
		RepoMetadataTypes: ctx.FormStrings("metadata_type"),
		RepoLanguageIsGL:  ctx.FormOptionalBool("is_gl"),
		// END DCS Customizations
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok":   true,
		"data": convert.ToUsers(ctx, ctx.Doer, users),
	})
}

// GetInfo get user's information
func GetInfo(ctx *context.APIContext) {
	// swagger:operation GET /users/{username} user userGet
	// ---
	// summary: Get a user
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/User"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !user_model.IsUserVisibleToViewer(ctx, ctx.ContextUser, ctx.Doer) {
		// fake ErrUserNotExist error message to not leak information about existence
		ctx.NotFound("GetUserByName", user_model.ErrUserNotExist{Name: ctx.Params(":username")})
		return
	}
	ctx.JSON(http.StatusOK, convert.ToUser(ctx, ctx.ContextUser, ctx.Doer))
}

// GetAuthenticatedUser get current user's information
func GetAuthenticatedUser(ctx *context.APIContext) {
	// swagger:operation GET /user user userGetCurrent
	// ---
	// summary: Get the authenticated user
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/User"

	ctx.JSON(http.StatusOK, convert.ToUser(ctx, ctx.Doer, ctx.Doer))
}

// GetUserHeatmapData is the handler to get a users heatmap
func GetUserHeatmapData(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/heatmap user userGetHeatmapData
	// ---
	// summary: Get a user's heatmap
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserHeatmapData"
	//   "404":
	//     "$ref": "#/responses/notFound"

	heatmap, err := activities_model.GetUserHeatmapDataByUser(ctx.ContextUser, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserHeatmapDataByUser", err)
		return
	}
	ctx.JSON(http.StatusOK, heatmap)
}

func ListUserActivityFeeds(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/activities/feeds user userListActivityFeeds
	// ---
	// summary: List a user's activity feeds
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: only-performed-by
	//   in: query
	//   description: if true, only show actions performed by the requested user
	//   type: boolean
	// - name: date
	//   in: query
	//   description: the date of the activities to be found
	//   type: string
	//   format: date
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityFeedsList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	includePrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == ctx.ContextUser.ID)
	listOptions := utils.GetListOptions(ctx)

	opts := activities_model.GetFeedsOptions{
		RequestedUser:   ctx.ContextUser,
		Actor:           ctx.Doer,
		IncludePrivate:  includePrivate,
		OnlyPerformedBy: ctx.FormBool("only-performed-by"),
		Date:            ctx.FormString("date"),
		ListOptions:     listOptions,
	}

	feeds, count, err := activities_model.GetFeeds(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetFeeds", err)
		return
	}
	ctx.SetTotalCountHeader(count)

	ctx.JSON(http.StatusOK, convert.ToActivities(ctx, feeds, ctx.Doer))
}
