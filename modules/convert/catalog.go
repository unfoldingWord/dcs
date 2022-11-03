// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToIngredient converts a Door43Metadata project to an api.Ingredient
func ToIngredient(project map[string]interface{}) *api.Ingredient {
	ingredient := &api.Ingredient{}
	fmt.Printf("PROJECT: %v\n\n\n\n", project)
	if val, ok := project["categories"].([]string); ok {
		ingredient.Categories = val
	}
	if val, ok := project["identifier"].(string); ok {
		ingredient.Identifier = val
	}
	if val, ok := project["path"].(string); ok {
		ingredient.Path = val
	}
	if val, ok := project["sort"].(int64); ok {
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
	if err := dm.LoadAttributes(); err != nil {
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

	var language string
	if val, ok := (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string); ok {
		language = val
	}

	languageDir := "ltr"
	if val, ok := (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["direction"].(string); ok {
		languageDir = val
	} else if language != "" {
		dcs.GetLanguageDirection(language)
	}

	var languageTitle string
	if val, ok := (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["title"].(string); ok {
		languageTitle = val
	} else if language != "" {
		dcs.GetLanguageTitle(language)
	}

	var languageIsGL bool
	if val, ok := (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["is_gl"].(bool); ok {
		languageIsGL = val
	}

	var books []string
	if val, ok := (*dm.Metadata)["books"].([]string); ok {
		books = val
	} else {
		books = dm.GetBooks()
	}

	var alignmentCounts map[string]int64
	if val, ok := (*dm.Metadata)["alignment_counts"]; ok {
		// Marshal/Unmarshal to let Unmarshaliing convert interface{} to map[string]int64
		if byteData, err := json.Marshal(val); err == nil {
			if err := json.Unmarshal(byteData, &alignmentCounts); err != nil {
				log.Error("Unable to Unmarshal alignment_counts: %v\n", val)
			}
		}
	}

	var ingredients []*api.Ingredient
	if val, ok := (*dm.Metadata)["projects"].([]interface{}); ok {
		ingredients = make([]*api.Ingredient, len(val))
		for i, project := range val {
			// Marshal/Unmarshal to let Unmarshaliing convert interface{} to Ingredient
			if byteData, err := json.Marshal(project); err == nil {
				err = json.Unmarshal(byteData, &ingredients[i])
				if err != nil {
					// Attempt our own conversation since Marshal/Unmarshal failed
					ingredients[i] = ToIngredient(project.(map[string]interface{}))
				}
			}
		}
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
		Language:               language,
		LanguageTitle:          languageTitle,
		LanguageDir:            languageDir,
		LanguageIsGL:           languageIsGL,
		Subject:                (*dm.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string),
		Title:                  (*dm.Metadata)["dublin_core"].(map[string]interface{})["title"].(string),
		Books:                  books,
		AlignmentCounts:        alignmentCounts,
		BranchOrTag:            dm.BranchOrTag,
		Stage:                  dm.Stage.String(),
		Released:               dm.ReleaseDateUnix.AsTime(),
		MetadataVersion:        dm.MetadataVersion,
		MetadataURL:            dm.GetMetadataURL(),
		MetadataJSONURL:        dm.GetMetadataJSONURL(),
		MetadataAPIContentsURL: dm.GetMetadataAPIContentsURL(),
		Ingredients:            ingredients,
	}
}
