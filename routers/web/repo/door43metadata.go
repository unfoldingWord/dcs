// Copyright 2023 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	door43metadata_service "code.gitea.io/gitea/services/door43metadata"

	"xorm.io/builder"
)

const (
	tplDCSHealthcheck base.TplName = "repo/dcs_healthcheck"
	tplDCSMetadata    base.TplName = "repo/dcs_metadata"
)

// GetRepoHealthcheck renders healthcheck for a repo
func GetRepoHealthcheck(ctx *context.Context) {
	dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, ctx.Repo.Repository.ID, ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		log.Error("Error getting door43 metadata for healthcheck: %v", err)
	}
	if dm != nil {
		dm.LoadHealthcheck(ctx)
	}
	ctx.Data["Title"] = "Health Check"
	ctx.Data["PageIsMetadata"] = true
	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["DefaultBranchDM"] = dm
	ctx.Data["Door43Metadatas"] = []*repo_model.Door43Metadata{dm, ctx.Repo.Repository.LatestProdDM}
	ctx.HTML(http.StatusOK, tplDCSHealthcheck)
}

// GetAllRepoDoor43Metadata renders all the door43metadatas for a repo
func GetAllRepoDoor43Metadata(ctx *context.Context) {
	branchDms := make([]*repo_model.Door43Metadata, 0, 50)
	err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": ctx.Repo.Repository.ID}).
		And(builder.Gte{"stage": door43metadata.StageLatest}).
		OrderBy("is_repo_metadata DESC, stage ASC, release_date_unix DESC").
		Find(&branchDms)
	if err != nil {
		log.Error("Find(dms) for branches: %v", err)
	}

	releaseDms := make([]*repo_model.Door43Metadata, 0, 50)
	err = db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": ctx.Repo.Repository.ID}).
		And(builder.Lte{"stage": door43metadata.StagePreProd}).
		OrderBy("release_date_unix DESC").
		Find(&releaseDms)
	if err != nil {
		log.Error("Find(dms) for releases: %v", err)
	}

	ctx.Data["Title"] = "Door43 Metadata"
	ctx.Data["PageIsMetadata"] = true
	ctx.Data["BranchDMs"] = branchDms
	ctx.Data["ReleaseDMs"] = releaseDms
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
