// Copyright 2023 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	go_context "context"
	"net/http"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
	door43metadata_service "code.gitea.io/gitea/services/door43metadata"

	"xorm.io/builder"
)

const (
	tplDCSMetadata    templates.TplName = "repo/dcs_metadata"
	tplDCSMetadataAll templates.TplName = "repo/dcs_metadata_all"
	tplDCSHealthcheck templates.TplName = "repo/dcs_healthcheck"

	releaseDMsPerPage = 20
)

// GetRepoMetadata renders the metadata summary page (default branch + latest release)
func GetRepoMetadata(ctx *context.Context) {
	_ = ctx.Repo.Repository.LoadLatestDMs(ctx)
	door43Metadatas := []*repo_model.Door43Metadata{}
	if ctx.Repo.Repository.RepoDM != nil && ctx.Repo.Repository.RepoDM.ID > 0 {
		door43Metadatas = append(door43Metadatas, ctx.Repo.Repository.RepoDM)
	}
	if ctx.Repo.Repository.LatestProdDM != nil {
		door43Metadatas = append(door43Metadatas, ctx.Repo.Repository.LatestProdDM)
	}

	ctx.Data["Title"] = "Metadata"
	ctx.Data["PageIsMetadata"] = true
	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["Door43Metadatas"] = door43Metadatas
	ctx.Data["IsRC"] = ctx.Repo.Repository.RepoDM != nil && ctx.Repo.Repository.RepoDM.MetadataType == "rc"
	ctx.HTML(http.StatusOK, tplDCSMetadata)
}

// GetRepoHealthcheck renders the health check page (RC repos only)
func GetRepoHealthcheck(ctx *context.Context) {
	_ = ctx.Repo.Repository.LoadLatestDMs(ctx)

	// Redirect non-RC repos to metadata page
	if ctx.Repo.Repository.RepoDM == nil || ctx.Repo.Repository.RepoDM.MetadataType != "rc" {
		ctx.Redirect(ctx.Repo.RepoLink + "/metadata")
		return
	}

	ctx.Data["Title"] = "Health Check"
	ctx.Data["PageIsHealthcheck"] = true
	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["IsRC"] = true
	ctx.HTML(http.StatusOK, tplDCSHealthcheck)
}

// GetAllRepoDoor43Metadata renders all door43metadatas for a repo with paginated releases
func GetAllRepoDoor43Metadata(ctx *context.Context) {
	_ = ctx.Repo.Repository.LoadLatestDMs(ctx)

	// Branches: load all (typically < 20)
	branchDms := make([]*repo_model.Door43Metadata, 0, 50)
	err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": ctx.Repo.Repository.ID}).
		And(builder.Eq{"ref_type": "branch"}).
		OrderBy("is_repo_metadata DESC, stage ASC, release_date_unix DESC").
		Find(&branchDms)
	if err != nil {
		log.Error("Find(dms) for branches: %v", err)
	}

	// Releases: paginated
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	releaseCount, err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": ctx.Repo.Repository.ID}).
		And(builder.Eq{"ref_type": "tag"}).
		Count(&repo_model.Door43Metadata{})
	if err != nil {
		log.Error("Count(dms) for releases: %v", err)
	}

	releaseDms := make([]*repo_model.Door43Metadata, 0, releaseDMsPerPage)
	err = db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": ctx.Repo.Repository.ID}).
		And(builder.Eq{"ref_type": "tag"}).
		OrderBy("release_date_unix DESC").
		Limit(releaseDMsPerPage, (page-1)*releaseDMsPerPage).
		Find(&releaseDms)
	if err != nil {
		log.Error("Find(dms) for releases: %v", err)
	}

	ctx.Data["Title"] = "Door43 Metadata"
	ctx.Data["PageIsMetadata"] = true
	ctx.Data["BranchDMs"] = branchDms
	ctx.Data["ReleaseDMs"] = releaseDms
	ctx.Data["ReleaseCount"] = releaseCount
	ctx.Data["IsRC"] = ctx.Repo.Repository.RepoDM != nil && ctx.Repo.Repository.RepoDM.MetadataType == "rc"

	pager := context.NewPagination(int(releaseCount), releaseDMsPerPage, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplDCSMetadataAll)
}

// UpdateDoor43Metadata updates the repo's metadata
func UpdateDoor43Metadata(ctx *context.Context) {
	runBackgroundTask := false
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, ctx.Repo.Repository, ctx.Repo.Repository.DefaultBranch); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: Error processing metadata [%s/%s]: %v", ctx.Repo.Repository.FullName(), ctx.Repo.Repository.DefaultBranch, err)
		ctx.Flash.Error("Error processing repo's metadata. Please contact the administrator.")
	} else {
		if err := ctx.Repo.Repository.LoadLatestDMs(ctx); err != nil {
			log.Error("LoadLatestDMs [%s] Error: %v", ctx.Repo.Repository.FullName(), err)
			ctx.Flash.Warning("Error loading metadata. Please try again.")
		} else if ctx.Repo.Repository.RepoDM.Metadata != nil {
			ctx.Flash.Success("Scanning of metadata for all branches and releases started. Reload page to see the updates as the metadata is populated.")
			runBackgroundTask = true
		} else {
			ctx.Flash.Warning("No metadata found!")
		}
	}

	if runBackgroundTask {
		go func(ctx go_context.Context, repo *repo_model.Repository) {
			select {
			case <-ctx.Done():
				log.Warn("ProcessDoor43MetadataForRepo: Context canceled [%s]", repo.FullName())
				return
			default:
				if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
					log.Error("ProcessDoor43MetadataForRepo: Error processing metadata [%s]: %v", repo, err)
				}
			}
		}(graceful.GetManager().ShutdownContext(), ctx.Repo.Repository)
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/metadata")
}
