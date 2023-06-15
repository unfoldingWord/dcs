// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/dcs"
	api "code.gitea.io/gitea/modules/structs"
)

func toRepoDCS(repo *repo_model.Repository, apiRepo *api.Repository) *api.Repository {
	apiRepo.Title = repo.Title
	apiRepo.Subject = repo.Subject
	apiRepo.Language = repo.Language
	apiRepo.LanguageDir = repo.LanguageDirection
	if apiRepo.LanguageDir == "" && apiRepo.Language != "" {
		apiRepo.LanguageDir = dcs.GetLanguageDirection(apiRepo.Language)
	}
	apiRepo.LanguageTitle = repo.LanguageTitle
	if apiRepo.LanguageTitle == "" && apiRepo.Language != "" {
		apiRepo.LanguageTitle = dcs.GetLanguageTitle(apiRepo.Language)
	}
	apiRepo.LanguageIsGL = repo.LanguageIsGL
	apiRepo.CheckingLevel = repo.CheckingLevel
	apiRepo.ContentFormat = repo.ContentFormat
	apiRepo.MetadataType = repo.MetadataType
	apiRepo.MetadataVersion = repo.MetadataVersion
	apiRepo.Ingredients = repo.Ingredients

	if apiRepo.Subject == "" {
		apiRepo.Subject = dcs.GetSubjectFromRepoName(repo.LowerName)
	}
	if apiRepo.Language == "" {
		apiRepo.Language = dcs.GetLanguageFromRepoName(repo.LowerName)
		apiRepo.LanguageIsGL = dcs.LanguageIsGL(apiRepo.Language)
	}
	if apiRepo.LanguageTitle == "" {
		apiRepo.LanguageTitle = dcs.GetLanguageTitle(apiRepo.Language)
	}
	if apiRepo.LanguageDir == "" {
		apiRepo.LanguageDir = dcs.GetLanguageDirection(apiRepo.Language)
	}

	apiRepo.CatalogStages = &api.CatalogStages{
		Production:    ToCatalogStage(repo.GetLatestProdDm()),
		PreProduction: ToCatalogStage(repo.GetLatestPreprodDm()),
		Latest:        ToCatalogStage(repo.GetDefaultBranchDm()),
	}

	return apiRepo
}
