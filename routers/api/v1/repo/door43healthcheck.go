// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/services/context"
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

	ctx.Repo.Repository.LoadLatestDMs(ctx)

	if ctx.Repo.Repository.RepoDM == nil || ctx.Repo.Repository.RepoDM.ID == 0 {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("no metadata found for repo [%s]", ctx.Repo.Repository.FullName()),
		})
		return
	}

	if ctx.Repo.Repository.RepoDM.MetadataType != "rc" {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"error": "currently only repositories of the 'rc' metadata type are supported",
		})
		return
	}

	healthcheck := ctx.Repo.Repository.RepoDM.GetHealthcheck(ctx)

	if healthcheck == nil {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("unable to perform a healthcheck on this repository [%s]", ctx.Repo.Repository.FullName()),
		})
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"ok":   true,
		"data": healthcheck,
	})
}
