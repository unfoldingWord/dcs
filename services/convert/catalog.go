// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToIngredient converts a Door43Metadata project to an api.Ingredient
func ToIngredient(project map[string]interface{}) *api.Ingredient {
	ingredient := &api.Ingredient{}
	if val, ok := project["categories"].([]string); ok {
		ingredient.Categories = val
	}
	if val, ok := project["identifier"].(string); ok {
		ingredient.Identifier = val
	}
	if val, ok := project["path"].(string); ok {
		ingredient.Path = val
	}
	if val, ok := project["sort"].(int); ok {
		ingredient.Sort = val
	}
	if val, ok := project["title"].(string); ok {
		ingredient.Title = val
	}
	if val, ok := project["versification"].(string); ok {
		ingredient.Versification = val
	}
	return ingredient
}

// ToCatalogEntry converts a Door43Metadata to an api.CatalogEntry
func ToCatalogEntry(ctx context.Context, dm *repo.Door43Metadata, perm access_model.Permission) *api.CatalogEntry {
	if err := dm.LoadRepo(ctx); err != nil {
		log.Error("ToCatalogEntry: dm.LoadAttributes() ERROR: %v", err)
		return nil
	}

	if err := dm.Repo.LoadOwner(ctx); err != nil {
		log.Error("ToCatalogEntry: dm.Repo.GetOwner() ERROR: %v", err)
		return nil
	}

	var release *api.Release
	if dm.Release != nil {
		release = ToAPIRelease(ctx, dm.Repo, dm.Release)
	}

	return &api.CatalogEntry{
		ID:                     dm.ID,
		Self:                   dm.APIURL(),
		Name:                   dm.Repo.Name,
		Owner:                  dm.Repo.OwnerName,
		FullName:               dm.Repo.FullName(),
		Repo:                   innerToRepo(ctx, dm.Repo, perm, true),
		Release:                release,
		TarballURL:             dm.GetTarballURL(),
		ZipballURL:             dm.GetZipballURL(),
		GitTreesURL:            dm.GetGitTreesURL(),
		ContentsURL:            dm.GetContentsURL(),
		Ref:                    dm.Ref,
		RefType:                dm.RefType,
		CommitSHA:              dm.CommitSHA,
		Language:               dm.Language,
		LanguageTitle:          dm.LanguageTitle,
		LanguageDir:            dm.LanguageDirection,
		LanguageIsGL:           dm.LanguageIsGL,
		Subject:                dm.Subject,
		Resource:               dm.Resource,
		Title:                  dm.Title,
		Stage:                  dm.Stage.String(),
		Released:               dm.ReleaseDateUnix.AsTime(),
		MetadataType:           dm.MetadataType,
		MetadataVersion:        dm.MetadataVersion,
		MetadataURL:            dm.GetMetadataURL(),
		MetadataJSONURL:        dm.GetMetadataJSONURL(),
		MetadataAPIContentsURL: dm.GetMetadataAPIContentsURL(),
		Ingredients:            dm.Ingredients,
		Books:                  dm.GetIngredientsIdentifierList(),
		ContentFormat:          dm.ContentFormat,
	}
}

// ToCatalogStage converts a Door43Metadata to an api.CatalogStage
func ToCatalogStage(ctx context.Context, dm *repo.Door43Metadata) *api.CatalogStage {
	if dm == nil {
		return nil
	}
	_ = dm.LoadAttributes(ctx)
	catalogStage := &api.CatalogStage{
		Ref:         dm.Ref,
		Released:    dm.ReleaseDateUnix.AsTime(),
		CommitSHA:   dm.CommitSHA,
		ZipballURL:  dm.GetZipballURL(),
		TarballURL:  dm.GetTarballURL(),
		GitTreesURL: dm.GetGitTreesURL(),
		ContentsURL: dm.GetContentsURL(),
	}
	url := dm.GetReleaseURL(ctx)
	if url != "" {
		catalogStage.ReleaseURL = &url
	}
	return catalogStage
}
