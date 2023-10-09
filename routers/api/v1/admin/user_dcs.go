// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
	user_service "code.gitea.io/gitea/services/user"
)

// ListSpamUsers API for getting all users considered to be spam
func ListSpamUsers(ctx *context.APIContext) {
	// swagger:operation GET /admin/users/spam admin adminListSpamUsers
	// ---
	// summary: List all users considered to be spam. NOTE - not all will be deleted in the DELETE action. see its description
	// produces:
	// - application/json
	// parameters:
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
	//     "$ref": "#/responses/UserList"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	listOptions := utils.GetListOptions(ctx)

	users, maxResults, err := user_model.SearchUsers(&user_model.SearchUserOptions{
		Actor:       ctx.Doer,
		Type:        user_model.UserTypeIndividual,
		OrderBy:     db.SearchUserOrderByAlphabetically,
		IsSpamUser:  util.OptionalBoolTrue,
		ListOptions: listOptions,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListSpamUsers", err)
		return
	}

	results := make([]*api.User, len(users))
	for i := range users {
		results[i] = convert.ToUser(ctx, users[i], ctx.Doer)
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &results)
}

// DeleteSpamUsers api for deleting all spam users
func DeleteSpamUsers(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/users/spam admin adminDeleteSpamUsers
	// ---
	// summary: Delete spam users - deletes those listed in the spam users list, but WILL NOT delete those that logged in more than 2 days from signing up, have repos, or was created in the last week.
	// produces:
	// - application/json
	// parameters:
	// - name: purge
	//   in: query
	//   description: purge the users from the system completely
	//   type: boolean
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	users := make([]*user_model.User, 0)
	err := db.GetEngine(ctx).
		OrderBy("id").
		Where("type = ?", user_model.UserTypeIndividual).
		And("TIMESTAMPDIFF(DAY, FROM_UNIXTIME(created_unix),  FROM_UNIXTIME(last_login_unix)) <= 2").
		And("description != ''").
		And("website != ''").
		And("num_repos = 0").
		And("last_login_unix < UNIX_TIMESTAMP(NOW() - INTERVAL 1 WEEK)").Find(&users)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteSpamUsers", err)
		return
	}

	for _, user := range users {
		if err := user_service.DeleteUser(ctx, user, ctx.FormBool("purge")); err != nil {
			if models.IsErrUserOwnRepos(err) ||
				models.IsErrUserHasOrgs(err) ||
				models.IsErrUserOwnPackages(err) {
				ctx.Error(http.StatusUnprocessableEntity, "", err)
			} else {
				ctx.Error(http.StatusInternalServerError, "DeleteUser", err)
			}
			return
		}
		log.Trace("Account deleted by admin(%s) due to being spam: %s", ctx.Doer.Name, user.Name)
	}

	ctx.Status(http.StatusNoContent)
}
