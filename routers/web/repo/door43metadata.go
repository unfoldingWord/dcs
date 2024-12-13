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
	tplDCSMetadata base.TplName = "repo/dcs_healthcheck"
)

// Healthcheck renders healthcheck and door43metadata page
func Healthcheck(ctx *context.Context) {
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

	var healthcheck *repo_model.HealthcheckGroupedIssues
	dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, ctx.Repo.Repository.ID, ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		log.Error("Error getting door43 metadata for healthcheck: %v", err)
	}
	if dm != nil {
		dm.LoadRepo(ctx)
		dm.Repo.LoadLatestDMs(ctx)
		if true || dm.HealthchckUnix == 0 || (dm.Repo.LatestProdDM != nil && (dm.Repo.LatestProdDM.HealthchckUnix == 0 || dm.Repo.LatestProdDM.ReleaseDateUnix > dm.HealthchckUnix)) {
			healthcheck, err = door43metadata_service.PerformHealthcheck(ctx, dm)
			if err != nil {
				log.Error("Error performing healthcheck: %v", err)
			}
		} else {
			healthcheck, err = repo_model.GetHealthcheckGroupedIssues(ctx, dm.ID)
			if err != nil {
				log.Error("Error getting healthcheck issues for healthcheck: %v", err)
			}
		}
	}

	ctx.Data["PageIsMetadata"] = true
	ctx.Data["Title"] = "Health Check"
	ctx.Data["PageIsHealthcheck"] = true
	ctx.Data["BranchDMs"] = branchDms
	ctx.Data["ReleaseDMs"] = releaseDms
	ctx.Data["Healthcheck"] = healthcheck
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
