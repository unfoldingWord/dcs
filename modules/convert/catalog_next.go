// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/dcs"
	api "code.gitea.io/gitea/modules/structs"
)

// ToCatalogEntry converts a Door43Metadata to an api.CatalogEntry
func ToCatalogEntry(dm *models.Door43Metadata, mode perm.AccessMode) *api.CatalogEntry {
	if err := dm.LoadAttributes(); err != nil {
		return nil
	}

	if err := dm.Repo.GetOwner(db.DefaultContext); err != nil {
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
	} else {
		languageIsGL = dcs.LanguageIsGL(language)
	}

	var books []interface{}
	if val, ok := (*dm.Metadata)["books"].([]interface{}); ok {
		books = val
	}

	var alignmentCounts map[string]interface{}
	if val, ok := (*dm.Metadata)["alignment_counts"].(map[string]interface{}); ok {
		alignmentCounts = val
	}

	var ingredients []interface{}
	if val, ok := (*dm.Metadata)["projects"].([]interface{}); ok {
		ingredients = val
	}

	return &api.CatalogEntry{
		ID:                     dm.ID,
		Self:                   dm.APIURLV5(),
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
