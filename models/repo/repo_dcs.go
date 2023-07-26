// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/modules/dcs"
)

// LoadLatestDMs loads the latest DMs
func (repo *Repository) LoadLatestDMs(ctx context.Context) error {
	if repo.LatestProdDM == nil {
		dm := &Door43Metadata{RepoID: repo.ID, Stage: door43metadata.StageProd, IsLatestForStage: true}
		has, err := db.GetEngine(ctx).Desc("release_date_unix").Get(dm)
		if err != nil {
			return err
		}
		if has {
			repo.LatestProdDM = dm
		}
	}

	if repo.LatestPreprodDM == nil {
		dm := &Door43Metadata{RepoID: repo.ID, Stage: door43metadata.StagePreProd, IsLatestForStage: true}
		has, err := db.GetEngine(ctx).Desc("release_date_unix").Get(dm)
		if err != nil {
			return err
		}
		if has {
			repo.LatestPreprodDM = dm
		}
	}

	if repo.DefaultBranchDM == nil {
		dm := &Door43Metadata{RepoID: repo.ID, Stage: door43metadata.StageLatest, IsLatestForStage: true}
		has, err := db.GetEngine(ctx).Desc("release_date_unix").Get(dm)
		if err != nil {
			return err
		}
		if has {
			repo.DefaultBranchDM = dm
		}
	}

	if repo.RepoDM == nil {
		dm := &Door43Metadata{RepoID: repo.ID, IsRepoMetadata: true}
		has, err := db.GetEngine(ctx).Desc("release_date_unix").Get(dm)
		if err != nil {
			return err
		}
		if has && dm != nil {
			repo.RepoDM = dm
		} else {
			title := repo.Name
			subject := dcs.GetSubjectFromRepoName(repo.Name)
			lang := dcs.GetLanguageFromRepoName(repo.Name)
			langDir := dcs.GetLanguageDirection(lang)
			langTitle := dcs.GetLanguageTitle(lang)
			langIsGL := dcs.LanguageIsGL(lang)
			repo.RepoDM = &Door43Metadata{
				RepoID:            repo.ID,
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
