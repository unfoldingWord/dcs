// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	"gitea.dev/models/db"
	org_model "gitea.dev/models/organization"
	packages_model "gitea.dev/models/packages"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	api "gitea.dev/modules/structs"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	user_service "gitea.dev/services/user"
)

// ListSpamUsers API for getting all users considered to be spam
func ListSpamUsers(ctx *context.APIContext) {
	// swagger:operation GET /admin/users/spam admin adminListSpamUsers
	// ---
	// summary: List all users considered to be spam. (have a description & website, last logged in on the day they signed up, and is older than a week)
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	users, err := getSpamUsers(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	results := make([]*api.User, len(users))
	for i := range users {
		results[i] = convert.ToUser(ctx, users[i], ctx.Doer)
	}

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

	users, err := getSpamUsers(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	for _, user := range users {
		if err := user_service.DeleteUser(ctx, user, ctx.FormBool("purge")); err != nil {
			if repo_model.IsErrUserOwnRepos(err) ||
				org_model.IsErrUserHasOrgs(err) ||
				packages_model.IsErrUserOwnPackages(err) {
				ctx.APIError(http.StatusUnprocessableEntity, err.Error())
			} else {
				ctx.APIErrorInternal(err)
			}
			return
		}
		log.Info("Account deleted by admin(%s) due to being spam: %s", ctx.Doer.Name, user.Name)
	}

	ctx.Status(http.StatusNoContent)
}

func getSpamUsers(ctx *context.APIContext) ([]*user_model.User, error) {
	users := make([]*user_model.User, 0)
	err := db.GetEngine(ctx).
		OrderBy("id").
		Where("type = ?", user_model.UserTypeIndividual).
		And("TIMESTAMPDIFF(DAY, FROM_UNIXTIME(created_unix),  FROM_UNIXTIME(last_login_unix)) <= 2").
		And("description != ''").
		And("website != ''").
		And("num_repos = 0").
		And("last_login_unix < UNIX_TIMESTAMP(NOW() - INTERVAL 1 WEEK)").Find(&users)
	return users, err
}
