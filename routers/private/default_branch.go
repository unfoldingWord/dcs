// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"fmt"
	"net/http"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/private"
	gitea_context "gitea.dev/services/context"
	door43metadata_service "gitea.dev/services/door43metadata"
	repo_service "gitea.dev/services/repository"
)

// SetDefaultBranch updates the default branch
func SetDefaultBranch(ctx *gitea_context.PrivateContext) {
	ownerName := ctx.PathParam("owner")
	repoName := ctx.PathParam("repo")
	branch := ctx.PathParam("branch")

	ctx.Repo.Repository.DefaultBranch = branch
	if err := gitrepo.SetDefaultBranch(ctx, ctx.Repo.Repository, ctx.Repo.Repository.DefaultBranch); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}

	if err := repo_model.UpdateDefaultBranch(ctx, ctx.Repo.Repository); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}

	/*** DCS Customizations ***/
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, ctx.Repo.Repository, branch); err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]any{
			"Err": fmt.Sprintf("Unable to process default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	/*** END DCS Customizations ***/

	if err := repo_service.AddRepoToLicenseUpdaterQueue(&repo_service.LicenseUpdaterOptions{
		RepoID: ctx.Repo.Repository.ID,
	}); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}

	ctx.PlainText(http.StatusOK, "success")
}
