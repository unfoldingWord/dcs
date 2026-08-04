// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	"gitea.dev/services/gitref"
)

// GetGitAllRefs get ref or an list all the refs of a repository
func GetGitAllRefs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/refs repository repoListAllGitRefs
	// ---
	// summary: Get specified ref or filtered repository's refs
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	// #   "$ref": "#/responses/Reference" TODO: swagger doesn't support different output formats by ref
	//     "$ref": "#/responses/ReferenceList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	getGitRefsInternal(ctx, "")
}

// GetGitRefs get ref or an filteresd list of refs of a repository
func GetGitRefs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/refs/{ref} repository repoListGitRefs
	// ---
	// summary: Get specified ref or filtered repository's refs
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: ref
	//   in: path
	//   description: part or full name of the ref
	//   type: string
	//   required: true
	// responses:
	//   "200":
	// #   "$ref": "#/responses/Reference" TODO: swagger doesn't support different output formats by ref
	//     "$ref": "#/responses/ReferenceList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	getGitRefsInternal(ctx, ctx.PathParam("*"))
}

func getGitRefsInternal(ctx *context.APIContext, filter string) {
	refs, lastMethodName, err := utils.GetGitRefs(ctx, filter)
	if err != nil {
		ctx.APIErrorInternal(fmt.Errorf("%s: %w", lastMethodName, err))
		return
	}

	if len(refs) == 0 {
		ctx.APIErrorNotFound()
		return
	}

	apiRefs := make([]*api.Reference, len(refs))
	for i := range refs {
		apiRefs[i] = convert.ToGitRef(ctx.Repo.Repository, refs[i])
	}
	// If single reference is found and it matches filter exactly return it as object
	if len(apiRefs) == 1 && apiRefs[0].Ref == filter {
		ctx.JSON(http.StatusOK, &apiRefs[0])
		return
	}
	ctx.JSON(http.StatusOK, &apiRefs)
}

/*** DCS Customizations ***/

// CreateGitRef creates a git ref for a repository that points to a target commitish
func CreateGitRef(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/git/refs repository repoCreateGitRef
	// ---
	// summary: Create a reference
	// description: Creates a reference for your repository. You are unable to create new references for empty repositories,
	//             even if the commit SHA-1 hash used exists. Empty repositories are repositories without branches.
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateGitRefOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Reference"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     description: The git ref with the same name already exists.
	//   "422":
	//     description: Unable to form reference

	opt := web.GetForm(ctx).(*api.CreateGitRefOption)

	if ctx.Repo.GitRepo.IsReferenceExist(opt.RefName) {
		ctx.APIError(http.StatusConflict, "reference already exists: "+opt.RefName)
		return
	}

	commitID, err := ctx.Repo.GitRepo.GetRefCommitID(opt.Target)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIError(http.StatusNotFound, "target does not exist: "+opt.Target)
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	ref, err := gitref.UpdateReferenceWithChecks(ctx, opt.RefName, commitID)
	if err != nil {
		if git.IsErrInvalidRefName(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		} else if git.IsErrProtectedRefName(err) {
			ctx.APIError(http.StatusMethodNotAllowed, err.Error())
		} else if git.IsErrRefNotFound(err) {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Sprintf("unable to load reference [ref_name: %s]", opt.RefName))
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToGitRef(ctx.Repo.Repository, ref))
}

// UpdateGitRef updates a branch for a repository from a commit SHA
func UpdateGitRef(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/git/refs/{ref} repository repoUpdateGitRef
	// ---
	// summary: Update a reference
	// description:
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: ref
	//   in: path
	//   description: name of the ref to update
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateGitRefOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Reference"
	//   "404":
	//     "$ref": "#/responses/notFound"

	refName := "refs/" + ctx.PathParam("*")
	opt := web.GetForm(ctx).(*api.UpdateGitRefOption)

	if !ctx.Repo.GitRepo.IsReferenceExist(refName) {
		ctx.APIError(http.StatusNotFound, "reference does not exist: "+refName)
		return
	}

	commitID, err := ctx.Repo.GitRepo.GetRefCommitID(opt.Target)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIError(http.StatusNotFound, "target does not exist: "+opt.Target)
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	ref, err := gitref.UpdateReferenceWithChecks(ctx, refName, commitID)
	if err != nil {
		if git.IsErrInvalidRefName(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		} else if git.IsErrProtectedRefName(err) {
			ctx.APIError(http.StatusMethodNotAllowed, err.Error())
		} else if git.IsErrRefNotFound(err) {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Sprintf("unable to load reference [ref_name: %s]", refName))
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ToGitRef(ctx.Repo.Repository, ref))
}

// DeleteGitRef deletes a git ref for a repository that points to a target commitish
func DeleteGitRef(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/git/refs/{ref} repository repoDeleteGitRef
	// ---
	// summary: Delete a reference
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: ref
	//   in: path
	//   description: name of the ref to be deleted
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "405":
	//     "$ref": "#/responses/error"
	//   "409":
	//     "$ref": "#/responses/conflict"

	refName := "refs/" + ctx.PathParam("*")

	if !ctx.Repo.GitRepo.IsReferenceExist(refName) {
		ctx.APIError(http.StatusNotFound, "reference does not exist: "+refName)
		return
	}

	err := gitref.RemoveReferenceWithChecks(ctx, refName)
	if err != nil {
		if git.IsErrInvalidRefName(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		} else if git.IsErrProtectedRefName(err) {
			ctx.APIError(http.StatusMethodNotAllowed, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

/*** END DCS Customizations ***/
