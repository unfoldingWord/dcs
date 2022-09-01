// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
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
	//     "$ref": "#/responses/Reference"
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
	//     "$ref": "#/responses/Reference"
	//     "$ref": "#/responses/ReferenceList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	getGitRefsInternal(ctx, ctx.Params("*"))
}

func getGitRefsInternal(ctx *context.APIContext, filter string) {
	refs, lastMethodName, err := utils.GetGitRefs(ctx, filter)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, lastMethodName, err)
		return
	}

	if len(refs) == 0 {
		ctx.NotFound()
		return
	}

	apiRefs := make([]*api.Reference, len(refs))
	for i := range refs {
		apiRefs[i] = &api.Reference{
			Ref: refs[i].Name,
			URL: ctx.Repo.Repository.APIURL() + "/git/" + util.PathEscapeSegments(refs[i].Name),
			Object: &api.GitObject{
				SHA:  refs[i].Object.String(),
				Type: refs[i].Type,
				URL:  ctx.Repo.Repository.APIURL() + "/git/" + url.PathEscape(refs[i].Type) + "s/" + url.PathEscape(refs[i].Object.String()),
			},
		}
	}
	// If single reference is found and it matches filter exactly return it as object
	if len(apiRefs) == 1 && apiRefs[0].Ref == filter {
		ctx.JSON(http.StatusOK, &apiRefs[0])
		return
	}
	ctx.JSON(http.StatusOK, &apiRefs)
}

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

	if ctx.Repo.GitRepo.IsReferenceExist(opt.Ref) {
		ctx.Error(http.StatusConflict, "reference already exists:", fmt.Errorf("reference already exists: %s", opt.Ref))
		return
	}

	if err := updateReference(ctx, opt.Ref, opt.Target); err != nil {
		return
	}

	refs, err := ctx.Repo.GitRepo.GetRefsFiltered(opt.Ref)
	if err != nil {
		ctx.ServerError("GetRefsFiltered", err)
		return
	}
	if len(refs) != 1 {
		ctx.Error(http.StatusConflict, "no references found", fmt.Errorf("there was a problem creating the gif ref: %s", opt.Ref))
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToGitRef(ctx.Repo.Repository, refs[0]))
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

	ref := fmt.Sprintf("refs/%s", ctx.Params("*"))
	opt := web.GetForm(ctx).(*api.UpdateGitRefOption)

	if !ctx.Repo.GitRepo.IsReferenceExist(ref) {
		ctx.Error(http.StatusNotFound, "git ref does not exist:", fmt.Errorf("reference does not exist: %s", ref))
		return
	}

	if err := updateReference(ctx, ref, opt.Target); err != nil {
		return
	}

	refs, err := ctx.Repo.GitRepo.GetRefsFiltered(ref)
	if err != nil {
		ctx.ServerError("GetRefsFiltered", err)
		return
	}
	if len(refs) != 1 {
		ctx.Error(http.StatusConflict, "no references found", fmt.Errorf("there was a problem updating the reference: %s", ref))
		return
	}
	ctx.JSON(http.StatusOK, convert.ToGitRef(ctx.Repo.Repository, refs[0]))
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

	ref := fmt.Sprintf("refs/%s", ctx.Params("*"))

	if !ctx.Repo.GitRepo.IsReferenceExist(ref) {
		ctx.Error(http.StatusNotFound, "git ref does not exist:", fmt.Errorf("reference does not exist: %s", ref))
		return
	}

	if err := updateReference(ctx, ref, ""); err != nil {
		return
	}

	ctx.Status(http.StatusNoContent)
}

func updateReference(ctx *context.APIContext, ref, target string) error {
	if !strings.HasPrefix(ref, "refs/") {
		err := fmt.Errorf("reference must start with 'refs/'")
		ctx.Error(http.StatusUnprocessableEntity, "bad reference'", err)
		return err
	}

	if strings.HasPrefix(ref, "refs/pull/") {
		err := fmt.Errorf("refs/pull/* is read-only.")
		ctx.Error(http.StatusUnprocessableEntity, "reference is read-only'", err)
		return err
	}

	if !userCanModifyRef(ctx, ref) {
		err := fmt.Errorf("user not allowed to modify a this reference: %s", ref)
		ctx.Error(http.StatusMethodNotAllowed, "user not allowed", err)
		return err
	}

	if target != "" {
		commitID, err := ctx.Repo.GitRepo.GetRefCommitID(target)
		if err != nil {
			if git.IsErrNotExist(err) {
				err := fmt.Errorf("target does not exist: %s", target)
				ctx.Error(http.StatusNotFound, "target does not exist", err)
				return err
			}
			ctx.InternalServerError(err)
			return err
		}
		if err := ctx.Repo.GitRepo.SetReference(ref, commitID); err != nil {
			message := err.Error()
			prefix := fmt.Sprintf("exit status 128 - fatal: update_ref failed for ref '%s': ", ref)
			if strings.HasPrefix(message, prefix) {
				message = strings.TrimRight(strings.TrimPrefix(message, prefix), "\n")
				ctx.Error(http.StatusUnprocessableEntity, "reference update failed", message)
			} else {
				ctx.InternalServerError(err)
			}
			return err
		}
	} else if err := ctx.Repo.GitRepo.RemoveReference(ref); err != nil {
		ctx.InternalServerError(err)
		return err
	}
	return nil
}

func userCanModifyRef(ctx *context.APIContext, ref string) bool {
	refPrefix, refName := git.SplitRefName(ref)
	if refPrefix == "refs/tags/" {
		if protectedTags, err := models.GetProtectedTags(ctx.Repo.Repository.ID); err == nil {
			if isAllowed, err := models.IsUserAllowedToControlTag(protectedTags, refName, ctx.User.ID); err == nil {
				return isAllowed
			}
		}
		return false
	}
	if refPrefix == "refs/heads/" {
		if isProtected, err := models.IsProtectedBranch(ctx.Repo.Repository.ID, refName); err == nil {
			return !isProtected
		}
		return false
	}
	return true
}
