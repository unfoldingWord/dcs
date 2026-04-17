// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/door43healthcheck"
)

// GetHealthcheck returns a simple healthcheck response
func GetHealthcheck(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/healthcheck repository repoGetHealthcheck
	// ---
	// summary: Get the healthcheck of a repo in JSON format
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Door43Healthcheck"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	var (
		dm  *repo_model.Door43Metadata
		err error
	)
	ref := ctx.FormTrim("ref")
	if ref != "" {
		dm, err = repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, ctx.Repo.Repository.ID, ref)
		if err != nil {
			if !repo_model.IsErrDoor43MetadataNotExist(err) {
				ctx.APIErrorInternal(err)
				return
			}
			ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
				"ok":    false,
				"error": fmt.Sprintf("no metadata found for repo [%s] and ref [%s]", ctx.Repo.Repository.FullName(), ref),
			})
			return
		}
	} else {
		_ = ctx.Repo.Repository.LoadLatestDMs(ctx)
		dm = ctx.Repo.Repository.RepoDM
	}

	if dm == nil || dm.ID == 0 {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("no metadata found for repo [%s]", ctx.Repo.Repository.FullName()),
		})
		return
	}

	if dm.MetadataType != "rc" {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"ok":   false,
			"info": "currently only repositories of the 'rc' metadata type are supported",
		})
		return
	}

	hc := door43healthcheck.RunHealthcheck(ctx, dm)

	if hc == nil {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("unable to perform a healthcheck on this repository [%s]", ctx.Repo.Repository.FullName()),
		})
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"ok":   true,
		"data": hc,
	})
}
