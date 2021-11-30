// Copyright 2021 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

// ToCatalogV3 converts a Door43Metadata to a api.CatalogV3 entry
func ToCatalogV3Resource(dm *models.Door43Metadata) *api.CatalogV3Resource {
	if err := dm.LoadAttributes(); err != nil {
		return nil
	}

	if err := dm.Repo.GetOwner(); err != nil {
		return nil
	}

	issued := dm.Metadata.DublinCore.Issued
	modified := dm.Metadata.DublinCore.Modified

	return &api.CatalogV3Resource{
		Checking:    dm.Metadata.DublinCore.Checking,
		Comment:     dm.Metadata.DublinCore.Comment,
		Contributor: dm.Metadata.DublinCore.Contributor,
		Creator:     dm.Metadata.DublinCore.Creator,
		Description: dm.Metadata.DublinCore.Description,
		Formats:     dm.Metadata.DublinCore.Formats,
		Identifier:  dm.Metadata.DublinCore.Identifier,
		Issued:      issued.Local().In(setting.DefaultUILocation),
		Modified:    modified.Local().In(setting.DefaultUILocation),
		Projects:    dm.Metadata.DublinCore.Projects,
		Publisher:   dm.Metadata.DublinCore.Publisher,
		Relation:    dm.Metadata.DublinCore.Relation,
		Rights:      dm.Metadata.DublinCore.Rights,
		Source:      dm.Metadata.DublinCore.Source,
		Subject:     dm.Metadata.DublinCore.Subject,
		Title:       dm.Metadata.DublinCore.Title,
		Version:     dm.Metadata.DublinCore.Version,
	}
}

// ToCatalogV4 converts a Door43Metadata to a api.CatalogV4 entry
func ToCatalogV4(dm *models.Door43Metadata, mode models.AccessMode) *api.CatalogV4 {
	err := dm.LoadAttributes()
	if err != nil {
		log.Error("loadAttributes: %v", err)
		return nil
	}
	return &api.CatalogV4{
		ID:                     dm.ID,
		Self:                   dm.APIURLV4(),
		Repo:                   dm.Repo.Name,
		Owner:                  dm.Repo.OwnerName,
		RepoURL:                dm.Repo.APIURL(),
		ReleaseURL:             dm.GetReleaseURL(),
		TarballURL:             dm.GetTarballURL(),
		ZipballURL:             dm.GetZipballURL(),
		Language:               (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string),
		Subject:                (*dm.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string),
		Title:                  (*dm.Metadata)["dublin_core"].(map[string]interface{})["title"].(string),
		Books:                  dm.GetBooks(),
		BranchOrTag:            dm.BranchOrTag,
		Stage:                  dm.Stage.String(),
		Released:               dm.ReleaseDateUnix.AsTime(),
		MetadataVersion:        dm.MetadataVersion,
		MetadataURL:            dm.GetMetadataURL(),
		MetadataJSONURL:        dm.GetMetadataJSONURL(),
		MetadataAPIContentsURL: dm.GetMetadataAPIContentsURL(),
		Ingredients:            (*dm.Metadata)["projects"].([]interface{}),
	}
}

// ToCatalogV5 converts a Door43Metadata to a api.CatalogV5 entry
func ToCatalogV5(dm *models.Door43Metadata, mode models.AccessMode) *api.CatalogV5 {
	if err := dm.LoadAttributes(); err != nil {
		return nil
	}

	if err := dm.Repo.GetOwner(); err != nil {
		return nil
	}

	var release *api.Release
	if dm.Release != nil {
		release = ToRelease(dm.Release)
	}

	return &api.CatalogV5{
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
		Language:               (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string),
		Subject:                (*dm.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string),
		Title:                  (*dm.Metadata)["dublin_core"].(map[string]interface{})["title"].(string),
		Books:                  dm.GetBooks(),
		BranchOrTag:            dm.BranchOrTag,
		Stage:                  dm.Stage.String(),
		Released:               dm.ReleaseDateUnix.AsTime(),
		MetadataVersion:        dm.MetadataVersion,
		MetadataURL:            dm.GetMetadataURL(),
		MetadataJSONURL:        dm.GetMetadataJSONURL(),
		MetadataAPIContentsURL: dm.GetMetadataAPIContentsURL(),
		Ingredients:            (*dm.Metadata)["projects"].([]interface{}),
	}
}
