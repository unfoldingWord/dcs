// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/user"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/org"
)

func listUserOrgs(ctx *context.APIContext, u *user_model.User) {
	listOptions := utils.GetListOptions(ctx)
	showPrivate := ctx.IsSigned && (ctx.Doer.IsAdmin || ctx.Doer.ID == u.ID)

	opts := organization.FindOrgOptions{
		ListOptions:    listOptions,
		UserID:         u.ID,
		IncludePrivate: showPrivate,
	}
	orgs, err := organization.FindOrgs(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindOrgs", err)
		return
	}
	maxResults, err := organization.CountOrgs(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CountOrgs", err)
		return
	}

	apiOrgs := make([]*api.Organization, len(orgs))
	for i := range orgs {
		apiOrgs[i] = convert.ToOrganization(ctx, orgs[i])
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &apiOrgs)
}

// ListMyOrgs list all my orgs
func ListMyOrgs(ctx *context.APIContext) {
	// swagger:operation GET /user/orgs organization orgListCurrentUserOrgs
	// ---
	// summary: List the current user's organizations
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
	//     "$ref": "#/responses/OrganizationList"

	listUserOrgs(ctx, ctx.Doer)
}

// ListUserOrgs list user's orgs
func ListUserOrgs(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/orgs organization orgListUserOrgs
	// ---
	// summary: List a user's organizations
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
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

	listUserOrgs(ctx, ctx.ContextUser)
}

// GetUserOrgsPermissions get user permissions in organization
func GetUserOrgsPermissions(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/orgs/{org}/permissions organization orgGetUserPermissions
	// ---
	// summary: Get user permissions in organization
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrganizationPermissions"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	var o *user_model.User
	if o = user.GetUserByParamsName(ctx, ":org"); o == nil {
		return
	}

	op := api.OrganizationPermissions{}

	if !organization.HasOrgOrUserVisible(ctx, o, ctx.ContextUser) {
		ctx.NotFound("HasOrgOrUserVisible", nil)
		return
	}

	org := organization.OrgFromUser(o)
	authorizeLevel, err := org.GetOrgUserMaxAuthorizeLevel(ctx.ContextUser.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetOrgUserAuthorizeLevel", err)
		return
	}

	if authorizeLevel > perm.AccessModeNone {
		op.CanRead = true
	}
	if authorizeLevel > perm.AccessModeRead {
		op.CanWrite = true
	}
	if authorizeLevel > perm.AccessModeWrite {
		op.IsAdmin = true
	}
	if authorizeLevel > perm.AccessModeAdmin {
		op.IsOwner = true
	}

	op.CanCreateRepository, err = org.CanCreateOrgRepo(ctx.ContextUser.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CanCreateOrgRepo", err)
		return
	}

	ctx.JSON(http.StatusOK, op)
}

// GetAll return list of all public organizations
func GetAll(ctx *context.APIContext) {
	// swagger:operation Get /orgs organization orgGetAll
	// ---
	// summary: Get list of organizations
	// produces:
	// - application/json
	// parameters:
	// - name: lang
	//   in: query
	//   description: if the org has one or more repos with the given language(s), the org will be in the results. Multiple lang's are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: is_gl
	//   in: query
	//   description: if the org has one or more repos that is a gateway laguage, the org will be in the results
	//   type: boolean
	// - name: subject
	//   in: query
	//   description: if the org has one or more repos with the given subject(s), the org will be in the results. Multiple subjects are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [Aligned Bible,Aramaic Grammar,Bible,Greek Grammar,Greek Lexicon,Greek New Testament,Hebrew Grammar,Hebrew Old Testament,Hebrew-Aramaic Lexicon,OBS Study Notes,OBS Study Questions,OBS Translation Notes,OBS Translation Questions,Open Bible Stories,Study Notes,Study Questions,Training Library,Translation Academy,Translation Notes,Translation Questions,Translation Words,TSV Study Notes,TSV Study Questions,TSV Translation Notes,TSV Translation Questions,TSV Translation Words Links,TSV OBS Study Notes,TSV OBS Study Questions,TSV OBS Translation Notes,TSV OBS Translation Questions,TSV OBS Translation Words Links]
	// - name: metadataType
	//   in: query
	//   description: if the org has one or more repos with the given metadata type(s), the org will be in the results. Multiple metadata types are ORed.
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

	vMode := []api.VisibleType{api.VisibleTypePublic}
	if ctx.IsSigned {
		vMode = append(vMode, api.VisibleTypeLimited)
		if ctx.Doer.IsAdmin {
			vMode = append(vMode, api.VisibleTypePrivate)
		}
	}

	listOptions := utils.GetListOptions(ctx)

	publicOrgs, maxResults, err := user_model.SearchUsers(&user_model.SearchUserOptions{
		Actor:       ctx.Doer,
		ListOptions: listOptions,
		Type:        user_model.UserTypeOrganization,
		OrderBy:     db.SearchUserOrderByAlphabetically,
		Visible:     vMode,
		/*** DCS Customizations ***/
		RepoLanguages:     ctx.FormStrings("lang"),
		RepoSubjects:      ctx.FormStrings("subject"),
		RepoMetadataTypes: ctx.FormStrings("metadataType"),
		/*** END DCS Customizations ***/
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchOrganizations", err)
		return
	}
	orgs := make([]*api.Organization, len(publicOrgs))
	for i := range publicOrgs {
		orgs[i] = convert.ToOrganization(ctx, organization.OrgFromUser(publicOrgs[i]))
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &orgs)
}

// Create api for create organization
func Create(ctx *context.APIContext) {
	// swagger:operation POST /orgs organization orgCreate
	// ---
	// summary: Create an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
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
	if !ctx.Doer.CanCreateOrganization() {
		ctx.Error(http.StatusForbidden, "Create organization not allowed", nil)
		return
	}

	visibility := api.VisibleTypePublic
	if form.Visibility != "" {
		visibility = api.VisibilityModes[form.Visibility]
	}

	org := &organization.Organization{
		Name:                      form.UserName,
		FullName:                  form.FullName,
		Description:               form.Description,
		Website:                   form.Website,
		Location:                  form.Location,
		IsActive:                  true,
		Type:                      user_model.UserTypeOrganization,
		Visibility:                visibility,
		RepoAdminChangeTeamAccess: form.RepoAdminChangeTeamAccess,
	}
	if err := organization.CreateOrganization(org, ctx.Doer); err != nil {
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

	ctx.JSON(http.StatusCreated, convert.ToOrganization(ctx, org))
}

// Get get an organization
func Get(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org} organization orgGet
	// ---
	// summary: Get an organization
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Organization"

	if !organization.HasOrgOrUserVisible(ctx, ctx.Org.Organization.AsUser(), ctx.Doer) {
		ctx.NotFound("HasOrgOrUserVisible", nil)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToOrganization(ctx, ctx.Org.Organization))
}

// Edit change an organization's information
func Edit(ctx *context.APIContext) {
	// swagger:operation PATCH /orgs/{org} organization orgEdit
	// ---
	// summary: Edit an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization to edit
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/EditOrgOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Organization"
	form := web.GetForm(ctx).(*api.EditOrgOption)
	org := ctx.Org.Organization
	org.FullName = form.FullName
	org.Description = form.Description
	org.Website = form.Website
	org.Location = form.Location
	if form.Visibility != "" {
		org.Visibility = api.VisibilityModes[form.Visibility]
	}
	if form.RepoAdminChangeTeamAccess != nil {
		org.RepoAdminChangeTeamAccess = *form.RepoAdminChangeTeamAccess
	}
	if err := user_model.UpdateUserCols(ctx, org.AsUser(),
		"full_name", "description", "website", "location",
		"visibility", "repo_admin_change_team_access",
	); err != nil {
		ctx.Error(http.StatusInternalServerError, "EditOrganization", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToOrganization(ctx, org))
}

// Delete an organization
func Delete(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org} organization orgDelete
	// ---
	// summary: Delete an organization
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: organization that is to be deleted
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	if err := org.DeleteOrganization(ctx.Org.Organization); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteOrganization", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func ListOrgActivityFeeds(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/activities/feeds organization orgListActivityFeeds
	// ---
	// summary: List an organization's activity feeds
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the org
	//   type: string
	//   required: true
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

	includePrivate := false
	if ctx.IsSigned {
		if ctx.Doer.IsAdmin {
			includePrivate = true
		} else {
			org := organization.OrgFromUser(ctx.ContextUser)
			isMember, err := org.IsOrgMember(ctx.Doer.ID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "IsOrgMember", err)
				return
			}
			includePrivate = isMember
		}
	}

	listOptions := utils.GetListOptions(ctx)

	opts := activities_model.GetFeedsOptions{
		RequestedUser:  ctx.ContextUser,
		Actor:          ctx.Doer,
		IncludePrivate: includePrivate,
		Date:           ctx.FormString("date"),
		ListOptions:    listOptions,
	}

	feeds, count, err := activities_model.GetFeeds(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetFeeds", err)
		return
	}
	ctx.SetTotalCountHeader(count)

	ctx.JSON(http.StatusOK, convert.ToActivities(ctx, feeds, ctx.Doer))
}
