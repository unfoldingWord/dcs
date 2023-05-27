// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
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
func ToCatalogEntry(dm *repo.Door43Metadata, mode perm.AccessMode) *api.CatalogEntry {
	if err := dm.GetRepo(); err != nil {
		log.Error("ToCatalogEntry: dm.LoadAttributes() ERROR: %v", err)
		return nil
	}

	if err := dm.Repo.GetOwner(db.DefaultContext); err != nil {
		log.Error("ToCatalogEntry: dm.Repo.GetOwner() ERROR: %v", err)
		return nil
	}

	var release *api.Release
	if dm.Release != nil {
		release = ToRelease(dm.Release)
	}

	return &api.CatalogEntry{
		ID:                     dm.ID,
		Self:                   dm.APIURL(),
		Name:                   dm.Repo.Name,
		Owner:                  dm.Repo.OwnerName,
		FullName:               dm.Repo.FullName(),
		Repo:                   innerToRepo(dm.Repo, mode, true),
		Release:                release,
		TarballURL:             dm.GetTarballURL(),
		ZipballURL:             dm.GetZipballURL(),
		GitTreesURL:            dm.GetGitTreesURL(),
		ContentsURL:            dm.GetContentsURL(),
		Ref:                    dm.Ref,
		RefType:                dm.RefType,
		CommitID:               dm.CommitSHA,
		Language:               dm.Language,
		LanguageTitle:          dm.LanguageTitle,
		LanguageDir:            dm.LanguageDirection,
		LanguageIsGL:           dm.LanguageIsGL,
		Subject:                dm.Subject,
		Title:                  dm.Title,
		Stage:                  dm.Stage.String(),
		Released:               dm.ReleaseDateUnix.AsTime(),
		MetadataType:           dm.MetadataType,
		MetadataVersion:        dm.MetadataVersion,
		MetadataURL:            dm.GetMetadataURL(),
		MetadataJSONURL:        dm.GetMetadataJSONURL(),
		MetadataAPIContentsURL: dm.GetMetadataAPIContentsURL(),
		Ingredients:            dm.Ingredients,
		ContentFormat:          dm.ContentFormat,
	}
}

// ToCatalogStage converts a Door43Metadata to an api.CatalogStage
func ToCatalogStage(dm *repo.Door43Metadata) *api.CatalogStage {
	if dm == nil {
		return nil
	}
	_ = dm.LoadAttributes()
	catalogStage := &api.CatalogStage{
		Ref:         dm.Ref,
		Released:    dm.ReleaseDateUnix.AsTime(),
		ZipballURL:  dm.GetZipballURL(),
		TarballURL:  dm.GetTarballURL(),
		GitTreesURL: dm.GetGitTreesURL(),
		ContentsURL: dm.GetContentsURL(),
	}
	if dm.GetReleaseURL() != "" {
		url := dm.GetReleaseURL()
		catalogStage.ReleaseURL = &url
	}
	return catalogStage
}
