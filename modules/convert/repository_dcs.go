// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models/door43metadata"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

func toRepoDCS(repo *repo_model.Repository, apiRepo *api.Repository) *api.Repository {
	latestDMs, err := repo_model.GetLatestDoor43MetadatasByRepoID(repo.ID)
	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		log.Error("GetDoor43MetadataByRepoIDAndReleaseID: %v", err)
	}
	var dm *repo_model.Door43Metadata
	if len(latestDMs) > 0 {
		dm = latestDMs[len(latestDMs)-1]
		dm.Repo = repo
	}

	if dm != nil {
		apiRepo.Title = dm.Title
		apiRepo.Subject = dm.Subject
		apiRepo.Language = dm.Language
		apiRepo.LanguageDir = dm.LanguageDirection
		if apiRepo.LanguageDir == "" && apiRepo.Language != "" {
			apiRepo.LanguageDir = dcs.GetLanguageDirection(apiRepo.Language)
		}
		apiRepo.LanguageTitle = dm.LanguageTitle
		if apiRepo.LanguageTitle == "" && apiRepo.Language != "" {
			apiRepo.LanguageTitle = dcs.GetLanguageTitle(apiRepo.Language)
		}
		apiRepo.LanguageIsGL = dm.LanguageIsGL
		apiRepo.CheckingLevel = dm.CheckingLevel
		apiRepo.ContentFormat = dm.ContentFormat
		apiRepo.MetadataType = dm.MetadataType
		apiRepo.MetadataVersion = dm.MetadataVersion
		apiRepo.Ingredients = dm.Ingredients
	} else {
		apiRepo.Subject = dcs.GetSubjectFromRepoName(repo.LowerName)
		apiRepo.Language = dcs.GetLanguageFromRepoName(repo.LowerName)
		apiRepo.LanguageTitle = dcs.GetLanguageTitle(apiRepo.Language)
		apiRepo.LanguageDir = dcs.GetLanguageDirection(apiRepo.Language)
		apiRepo.LanguageIsGL = dcs.LanguageIsGL(apiRepo.Language)
		if repo.PrimaryLanguage != nil {
			apiRepo.ContentFormat = repo.PrimaryLanguage.Language
		}
	}

	if len(latestDMs) > 0 {
		apiRepo.CatalogStages = &api.CatalogStages{}
		for _, latestDM := range latestDMs {
			latestDM.Repo = repo
			catalogStage := ToCatalogStage(latestDM)
			switch latestDM.Stage {
			case door43metadata.StageProd:
				apiRepo.CatalogStages.Production = catalogStage
			case door43metadata.StagePreProd:
				apiRepo.CatalogStages.PreProduction = catalogStage
			case door43metadata.StageDraft:
				apiRepo.CatalogStages.Draft = catalogStage
			case door43metadata.StageLatest:
				apiRepo.CatalogStages.Latest = catalogStage
			}
		}
	}

	return apiRepo
}
