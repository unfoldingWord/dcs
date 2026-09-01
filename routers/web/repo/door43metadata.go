// Copyright 2023 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	go_context "context"
	"net/http"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	"gitea.dev/services/door43healthcheck"
	door43metadata_service "gitea.dev/services/door43metadata"

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
	ctx.HTML(http.StatusOK, tplDCSMetadata)
}

// GetRepoHealthcheck renders the health check page: /healthcheck shows the repo's
// canonical entry (default branch, falling back to the latest release), while
// /healthcheck/{ref} shows the given branch or tag's own health check.
func GetRepoHealthcheck(ctx *context.Context) {
	_ = ctx.Repo.Repository.LoadLatestDMs(ctx)

	var dm *repo_model.Door43Metadata
	ref := ctx.PathParam("*")
	if ref != "" {
		var err error
		dm, err = repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, ctx.Repo.Repository.ID, ref)
		if err != nil {
			if !repo_model.IsErrDoor43MetadataNotExist(err) {
				ctx.ServerError("GetDoor43MetadataByRepoIDAndRef", err)
				return
			}
			// no entry for this ref; fall through to the redirect below
			dm = nil
		}
	} else {
		dm = ctx.Repo.Repository.RepoDM
	}

	// Redirect refs/repos without a checkable entry to the metadata page
	if dm == nil || dm.ID == 0 || !door43healthcheck.Supported(dm.MetadataType) {
		ctx.Redirect(ctx.Repo.RepoLink + "/metadata")
		return
	}

	ctx.Data["Title"] = "Health Check"
	ctx.Data["PageIsHealthcheck"] = true
	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["HealthcheckDM"] = dm
	ctx.Data["HealthcheckRef"] = ref
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

	ctx.Data["Page"] = context.NewPagerBuilder(ctx).TotalCount(releaseCount).PerPageLimit(releaseDMsPerPage).CurPage(page).Build()

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
			// a panic while processing one repo must never take down the server
			defer func() {
				if err := recover(); err != nil {
					log.Error("ProcessDoor43MetadataForRepo: PANIC [%s]: %v\n%s", repo.FullName(), err, log.Stack(2))
				}
			}()
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
