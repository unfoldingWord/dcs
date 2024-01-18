// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/modules/dcs"

	"xorm.io/builder"
)

// LoadLatestDMs loads the latest DMs
func (repo *Repository) LoadLatestDMs(ctx context.Context) error {
	if repo.LatestProdDM == nil {
		dm := &Door43Metadata{}
		has, err := db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repo.ID}).
			And(builder.Eq{"stage": door43metadata.StageProd}).
			And(builder.Eq{"is_latest_for_stage": true}).
			And(builder.Eq{"is_invalid": false}).
			Desc("release_date_unix").
			Get(dm)
		if err != nil {
			return err
		}
		if has && dm != nil {
			repo.LatestProdDM = dm
		}
	}

	if repo.LatestPreprodDM == nil {
		dm := &Door43Metadata{}
		has, err := db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repo.ID}).
			And(builder.Eq{"stage": door43metadata.StagePreProd}).
			And(builder.Eq{"is_latest_for_stage": true}).
			And(builder.Eq{"is_invalid": false}).
			Desc("release_date_unix").
			Get(dm)
		if err != nil {
			return err
		}
		if has && dm != nil {
			repo.LatestPreprodDM = dm
		}
	}

	if repo.DefaultBranchDM == nil {
		dm := &Door43Metadata{}
		has, err := db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repo.ID}).
			And(builder.Eq{"stage": door43metadata.StageLatest}).
			And(builder.Eq{"is_latest_for_stage": true}).
			And(builder.Eq{"is_invalid": false}).
			Desc("release_date_unix").
			Get(dm)
		if err != nil {
			return err
		}
		if has && dm != nil {
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
		if has && dm != nil {
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
func (rl RepositoryList) LoadLatestDMs(ctx context.Context) error {
	if rl.Len() == 0 {
		return nil
	}
	var lastErr error
	for _, repo := range rl {
		if err := repo.LoadLatestDMs(ctx); err != nil && lastErr == nil {
			lastErr = err
		}
	}
	return lastErr
}
