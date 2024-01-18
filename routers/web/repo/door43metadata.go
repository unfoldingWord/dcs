// Copyright 2023 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	door43metadata_service "code.gitea.io/gitea/services/door43metadata"

	"xorm.io/builder"
)

const (
	tplDCSMetadata base.TplName = "repo/dcs_metadata"
	tplPreview     base.TplName = "repo/dcs_preview"
)

// Door43Metadtas renders door43 metadatas page
func Door43Metadatas(ctx *context.Context) {
	dms := make([]*repo_model.Door43Metadata, 0, 50)
	err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": ctx.Repo.Repository.ID}).
		OrderBy("is_repo_metadata DESC, ref_type ASC, release_date_unix DESC").
		Find(&dms)
	if err != nil {
		log.Error("Find(dms): %v", err)
	}

	ctx.Data["PageIsMetadata"] = true
	ctx.Data["Title"] = "Door43 Metadata"
	ctx.Data["PageIsSettingsDoor43Metadata"] = true
	ctx.Data["Door43Metadatas"] = dms
	ctx.HTML(http.StatusOK, tplDCSMetadata)
}

// UpdateDoor43Metadata updates the repo's metadata
func UpdateDoor43Metadata(ctx *context.Context) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, ctx.Repo.Repository, ""); err != nil {
		ctx.Flash.Error("ProcessDoor43MetadataForRepo: " + err.Error())
	} else {
		if err := ctx.Repo.Repository.LoadLatestDMs(ctx); err != nil {
			ctx.Flash.Warning("Error loading metadata. Please try again.")
		} else if ctx.Repo.Repository.RepoDM.Metadata != nil {
			ctx.Flash.Success("Successfully scanned this repo's metadata.")
		} else {
			ctx.Flash.Warning("No metadata found!")
		}
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/metadata")
}

// PreviewRepo creates a page for a react component app
func PreviewRepo(ctx *context.Context) {
	commitsCount, err := ctx.Repo.GetCommitsCount()
	if err != nil {
		ctx.ServerError("GetCommitsCount", err)
		return
	}
	ctx.Data["PageIsPreview"] = true
	ctx.Data["CommitsCount"] = commitsCount
	ctx.Data["Title"] = "Preview of " + ctx.Repo.Repository.FullName() + " - " + ctx.Repo.RefName
	ctx.HTML(http.StatusOK, tplPreview)
}
