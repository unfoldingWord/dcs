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
			title := repo.Name
			metadataType := dcs.GetMetadataTypeFromRepoName(repo.Name)
			metadataVersion := dcs.GetDefaultMetadataVersionForType(metadataType)
			subject := dcs.GetSubjectFromRepoName(repo.Name)
			lang := dcs.GetLanguageFromRepoName(repo.Name)
			langDir := dcs.GetLanguageDirection(lang)
			langTitle := dcs.GetLanguageTitle(lang)
			langIsGL := dcs.LanguageIsGL(lang)
			repo.RepoDM = &Door43Metadata{
				RepoID:            repo.ID,
				Repo:              repo,
				MetadataType:      metadataType,
				MetadataVersion:   metadataVersion,
				Title:             title,
				Subject:           subject,
				Language:          lang,
				LanguageDirection: langDir,
				LanguageTitle:     langTitle,
				LanguageIsGL:      langIsGL,
			}
		}
	}

	return nil
}

// LoadLatestDMs loads the latest Door43Metadatas for the given RepositoryList
func (repos RepositoryList) LoadLatestDMs(ctx context.Context) error {
	if repos.Len() == 0 {
		return nil
	}
	var lastErr error
	for _, repo := range repos {
		if err := repo.LoadLatestDMs(ctx); err != nil && lastErr == nil {
			lastErr = err
		}
	}
	return lastErr
}
