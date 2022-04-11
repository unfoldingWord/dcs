// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// CreateOrg api for create organization
func CreateOrg(ctx *context.APIContext) {
	// swagger:operation POST /admin/users/{username}/orgs admin adminCreateOrg
	// ---
	// summary: Create an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user that will own the created organization
	//   type: string
	//   required: true
	// - name: organization
	//   in: body
	//   required: true
	//   schema: { "$ref": "#/definitions/CreateOrgOption" }
	// responses:
	//   "201":
	//     "$ref": "#/responses/Organization"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateOrgOption)

	visibility := api.VisibleTypePublic
	if form.Visibility != "" {
		visibility = api.VisibilityModes[form.Visibility]
	}

	org := &organization.Organization{
		Name:        form.UserName,
		FullName:    form.FullName,
		Description: form.Description,
		Website:     form.Website,
		Location:    form.Location,
		IsActive:    true,
		Type:        user_model.UserTypeOrganization,
		Visibility:  visibility,
	}

	if err := organization.CreateOrganization(org, ctx.ContextUser); err != nil {
		if user_model.IsErrUserAlreadyExist(err) ||
			db.IsErrNameReserved(err) ||
			db.IsErrNameCharsNotAllowed(err) ||
			db.IsErrNamePatternNotAllowed(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateOrganization", err)
		}
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToOrganization(org))
}

// GetAllOrgs API for getting information of all the organizations
func GetAllOrgs(ctx *context.APIContext) {
	// swagger:operation GET /admin/orgs admin adminGetAllOrgs
	// ---
	// summary: List all organizations
	// produces:
	// - application/json
	// parameters:
	// - name: lang
	//   in: query
	//   description: If the org has one or more repos with the given language(s), the org will be in the results. Multiple lang's are ORed.
	//   type: string
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
	//     "$ref": "#/responses/OrganizationList"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	listOptions := utils.GetListOptions(ctx)

	users, maxResults, err := user_model.SearchUsers(&user_model.SearchUserOptions{
		Actor:       ctx.Doer,
		Type:        user_model.UserTypeOrganization,
		OrderBy:     db.SearchUserOrderByAlphabetically,
		ListOptions: listOptions,
		Visible:     []api.VisibleType{api.VisibleTypePublic, api.VisibleTypeLimited, api.VisibleTypePrivate},
		/*** DCS Customizations ***/
		RepoLanguages: ctx.FormStrings("lang"),
		/*** END DCS Customizations ***/
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchOrganizations", err)
		return
	}
	orgs := make([]*api.Organization, len(users))
	for i := range users {
		orgs[i] = convert.ToOrganization(organization.OrgFromUser(users[i]))
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &orgs)
}
