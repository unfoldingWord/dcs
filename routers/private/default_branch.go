// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"fmt"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/private"
	door43metadata_service "code.gitea.io/gitea/services/door43metadata"
)

// SetDefaultBranch updates the default branch
func SetDefaultBranch(ctx *gitea_context.PrivateContext) {
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	branch := ctx.Params(":branch")

	ctx.Repo.Repository.DefaultBranch = branch
	if err := ctx.Repo.GitRepo.SetDefaultBranch(ctx.Repo.Repository.DefaultBranch); err != nil {
		if !git.IsErrUnsupportedVersion(err) {
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
	}

	if err := repo_model.UpdateDefaultBranch(ctx, ctx.Repo.Repository); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}

	/*** DCS Customizations ***/
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, ctx.Repo.Repository, branch); err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"Err": fmt.Sprintf("Unable to process default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	/*** END DCS Customizations ***/

	ctx.PlainText(http.StatusOK, "success")
}
