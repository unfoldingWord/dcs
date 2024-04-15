// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRepoDCS adds Door43 metadata properties to the API Repo object
func ToRepoDCS(ctx context.Context, repo *repo_model.Repository, apiRepo *api.Repository) *api.Repository {
	if err := repo.LoadLatestDMs(ctx); err != nil {
		return apiRepo
	}
	dm := repo.RepoDM
	apiRepo.Title = dm.Title
	apiRepo.Subject = dm.Subject
	apiRepo.FlavorType = dm.FlavorType
	apiRepo.Flavor = dm.Flavor
	apiRepo.Abbreviation = dm.Abbreviation
	apiRepo.Language = dm.Language
	apiRepo.LanguageTitle = dm.LanguageTitle
	apiRepo.LanguageDir = dm.LanguageDirection
	apiRepo.LanguageIsGL = dm.LanguageIsGL
	apiRepo.CheckingLevel = dm.CheckingLevel
	apiRepo.ContentFormat = dm.ContentFormat
	apiRepo.MetadataType = dm.MetadataType
	apiRepo.MetadataVersion = dm.MetadataVersion
	apiRepo.Ingredients = dm.Ingredients
	apiRepo.CatalogStages = &api.CatalogStages{
		Production:    ToCatalogStage(ctx, repo.LatestProdDM),
		PreProduction: ToCatalogStage(ctx, repo.LatestPreprodDM),
		Latest:        ToCatalogStage(ctx, repo.DefaultBranchDM),
	}
	return apiRepo
}
