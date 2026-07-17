// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"net/url"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// HealthcheckURL the api url for a repo's healthcheck
func (repo *Repository) RepoHealthcheckURL() string {
	return setting.AppURL + "api/v1/repos/" + url.PathEscape(repo.OwnerName) + "/" + url.PathEscape(repo.Name) + "/healthcheck"
}

// LoadLatestDMs loads the latest DMs
func (repo *Repository) LoadLatestDMs(ctx context.Context) error {
	if repo.LatestDMsLoaded {
		return nil
	}
	if repo.LatestProdDM == nil {
		dm := &Door43Metadata{}
		has, err := db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repo.ID}).
			And(builder.Eq{"stage": door43metadata.StageProd}).
			And(builder.Eq{"is_latest_for_stage": true}).
			And(builder.IsNull{"validation_error"}).
			Desc("release_date_unix").
			Get(dm)
		if err != nil {
			return err
		}
		if has {
			dm.Repo = repo
			_ = dm.LoadAttributes(ctx)
			repo.LatestProdDM = dm
		}
	}

	if repo.LatestPreprodDM == nil {
		dm := &Door43Metadata{}
		has, err := db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repo.ID}).
			And(builder.Eq{"stage": door43metadata.StagePreProd}).
			And(builder.Eq{"is_latest_for_stage": true}).
			And(builder.IsNull{"validation_error"}).
			Desc("release_date_unix").
			Get(dm)
		if err != nil {
			return err
		}
		if has && dm.ID != 0 {
			dm.Repo = repo
			repo.LatestPreprodDM = dm
		}
	}

	if repo.DefaultBranchDM == nil {
		dm := &Door43Metadata{}
		has, err := db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repo.ID}).
			And(builder.Eq{"stage": door43metadata.StageLatest}).
			And(builder.Eq{"is_latest_for_stage": true}).
			And(builder.IsNull{"validation_error"}).
			Desc("release_date_unix").
			Get(dm)
		if err != nil {
			return err
		}
		if has && dm.ID != 0 {
			dm.Repo = repo
			repo.DefaultBranchDM = dm
		}
	}

	if repo.RepoDM == nil {
		dm := &Door43Metadata{}
		has, err := db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repo.ID}).
			And(builder.Eq{"is_repo_metadata": true}).
			Desc("release_date_unix").
			Get(dm)
		if err != nil {
			return err
		}
		if has && dm.ID != 0 {
			repo.RepoDM = dm
		} else {
			repo.RepoDM = SynthesizeRepoDM(repo)
		}
	}

	repo.LatestDMsLoaded = true
	return nil
}

// SynthesizeRepoDM builds the fallback RepoDM for a repo that has no
// is_repo_metadata Door43Metadata row, deriving fields from the repo name.
func SynthesizeRepoDM(repo *Repository) *Door43Metadata {
	metadataType := dcs.GetMetadataTypeFromRepoName(repo.Name)
	lang := dcs.GetLanguageFromRepoName(repo.Name)
	return &Door43Metadata{
		RepoID:            repo.ID,
		Repo:              repo,
		MetadataType:      metadataType,
		MetadataVersion:   dcs.GetDefaultMetadataVersionForType(metadataType),
		Title:             repo.Name,
		Subject:           dcs.GetSubjectFromRepoName(repo.Name),
		Language:          lang,
		LanguageDirection: dcs.GetLanguageDirection(lang),
		LanguageTitle:     dcs.GetLanguageTitle(lang),
		LanguageIsGL:      dcs.LanguageIsGL(lang),
	}
}

// LoadLatestDMs loads the latest Door43Metadatas for the given RepositoryList
// in two queries plus one batched attribute load, instead of 4+ queries per repo.
func (repos RepositoryList) LoadLatestDMs(ctx context.Context) error {
	if repos.Len() == 0 {
		return nil
	}

	repoMap := make(map[int64]*Repository, len(repos))
	ids := make([]int64, 0, len(repos))
	for _, repo := range repos {
		if !repo.LatestDMsLoaded {
			repoMap[repo.ID] = repo
			ids = append(ids, repo.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	var dms []*Door43Metadata
	err := db.GetEngine(ctx).
		In("repo_id", ids).
		And(builder.Or(
			builder.And(builder.Eq{"is_latest_for_stage": true}, builder.IsNull{"validation_error"}),
			builder.Eq{"is_repo_metadata": true},
		)).
		Desc("release_date_unix").
		Find(&dms)
	if err != nil {
		return err
	}

	loaded := make(Door43MetadataList, 0, len(dms))
	for _, dm := range dms {
		repo := repoMap[dm.RepoID]
		if repo == nil {
			continue
		}
		dm.Repo = repo
		used := false
		if dm.IsLatestForStage && dm.ValidationError == nil {
			// rows are ordered newest first, so only the first per stage sticks
			switch dm.Stage {
			case door43metadata.StageProd:
				if repo.LatestProdDM == nil {
					repo.LatestProdDM = dm
					used = true
				}
			case door43metadata.StagePreProd:
				if repo.LatestPreprodDM == nil {
					repo.LatestPreprodDM = dm
					used = true
				}
			case door43metadata.StageLatest:
				if repo.DefaultBranchDM == nil {
					repo.DefaultBranchDM = dm
					used = true
				}
			}
		}
		if dm.IsRepoMetadata && repo.RepoDM == nil {
			repo.RepoDM = dm
			used = true
		}
		if used {
			loaded = append(loaded, dm)
		}
	}

	for _, repo := range repoMap {
		if repo.RepoDM == nil {
			repo.RepoDM = SynthesizeRepoDM(repo)
		}
		repo.LatestDMsLoaded = true
	}

	// Batch-load releases/attachments/publishers of the stage DMs so the
	// per-DM conversions (e.g. ToCatalogStage) don't query per entry
	return loaded.LoadAttributes(ctx)
}
