// Copyright 2021 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

// ToCatalogV3Resource converts a Door43Metadata to a api.CatalogV3 resource entry
func ToCatalogV3Resource(dm *models.Door43Metadata) *api.CatalogV3Resource {
	if err := dm.LoadAttributes(); err != nil {
		return nil
	}

	if err := dm.Repo.GetOwner(); err != nil {
		return nil
	}

	issuedStr := (*dm.Metadata)["dublin_core"].(map[string]interface{})["issued"].(string)
	issued, err := time.Parse("2006-01-02", issuedStr)
	if err != nil {
		issued, err = time.Parse("20060102", issuedStr)
		if err != nil {
			issued = time.Time{}
		}
	}

	modifiedStr := (*dm.Metadata)["dublin_core"].(map[string]interface{})["modified"].(string)
	modified, err := time.Parse("2006-01-02", modifiedStr)
	if err != nil {
		modified, err = time.Parse("20060102", modifiedStr)
		if err != nil {
			modified = time.Time{}
		}
	}

	var checking map[string]interface{}
	if val, ok := (*dm.Metadata)["checking"]; ok && val != nil {
		checking = val.(map[string]interface{})
	}

	var comment string
	if val, ok := (*dm.Metadata)["dublin_core"].(map[string]interface{})["comment"]; ok && val != nil {
		comment = val.(string)
	}

	var contributor []interface{}
	if val, ok := (*dm.Metadata)["dublin_core"].(map[string]interface{})["contributor"]; ok && val != nil {
		contributor = val.([]interface{})
	}

	// TODO: GET THE WHOLE FORMAT AS WELL AS PDF FROM media.yaml
	var formats []map[string]interface{}
	var format string
	if val, ok := (*dm.Metadata)["dublin_core"].(map[string]interface{})["format"]; ok && val != nil {
		format = val.(string)
	} else {
		format = "text/markdown"
	}
	format = fmt.Sprintf("application/zip; type=%s content=%s conformsto=%s",
		(*dm.Metadata)["dublin_core"].(map[string]interface{})["type"].(string),
		format,
		(*dm.Metadata)["dublin_core"].(map[string]interface{})["conformsto"].(string))
	var formatModified time.Time
	var formatSize int64
	gitRepo, _ := git.OpenRepository(dm.Repo.RepoPath())
	if gitRepo != nil {
		defer gitRepo.Close()
		var commit *git.Commit
		if dm.ReleaseID == 0 {
			commit, _ = gitRepo.GetBranchCommit(dm.Repo.DefaultBranch)
		} else {
			commit, _ = gitRepo.GetTagCommit(dm.BranchOrTag)
		}
		if commit != nil {
			formatModified = commit.Author.When
		}
		// if ctx, commiter, err := models.TxDBContext(); err == nil {
		// 	defer commiter.Close()
		// 	if archiver, err := models.GetRepoArchiver(ctx, dm.Repo.ID, git.ZIP, commit.ID.String()); err == nil {
		// 		if rPath, err := archiver.RelativePath(); err == nil {
		// 			if info, err := storage.RepoArchives.Stat(rPath); err == nil {
		// 				formatSize = info.Size()
		// 			}
		// 		}
		// 	}
		// }
	}
	formatURL := util.URLJoin(dm.Repo.HTMLURL(), "archive", dm.BranchOrTag+".zip")
	// resp, err := http.Head(formatURL)
	// fmt.Printf("%s\n\n%v\n\n%v\n\n", formatURL, resp, err)
	// if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
	// 	fmt.Printf("HEADER:%v\n\nCL: %s\n\n", resp.Header, resp.Header.Get("Content-Length"))
	// 	size, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	// 	fmt.Printf("%d\n\n\n%v\n\n", size, err)
	// 	formatSize = int64(size)
	// }

	formats = append(formats, map[string]interface{}{
		"format":    format,
		"modified":  formatModified,
		"signature": "",
		"size":      formatSize,
		"url":       formatURL,
	})

	var projects []interface{}
	if val, ok := (*dm.Metadata)["projects"]; ok && val != nil {
		projects = val.([]interface{})
	}

	var relation []interface{}
	if val, ok := (*dm.Metadata)["dublin_core"].(map[string]interface{})["relation"]; ok && val != nil {
		relation = val.([]interface{})
	}

	return &api.CatalogV3Resource{
		Checking:    checking,
		Comment:     comment,
		Contributor: contributor,
		Creator:     (*dm.Metadata)["dublin_core"].(map[string]interface{})["creator"].(string),
		Description: (*dm.Metadata)["dublin_core"].(map[string]interface{})["description"].(string),
		Formats:     formats,
		Identifier:  (*dm.Metadata)["dublin_core"].(map[string]interface{})["identifier"].(string),
		Issued:      issued.Local().In(setting.DefaultUILocation),
		Modified:    modified.Local().In(setting.DefaultUILocation),
		Projects:    projects,
		Publisher:   (*dm.Metadata)["dublin_core"].(map[string]interface{})["publisher"].(string),
		Relation:    relation,
		Rights:      (*dm.Metadata)["dublin_core"].(map[string]interface{})["rights"].(string),
		Source:      (*dm.Metadata)["dublin_core"].(map[string]interface{})["source"].([]interface{}),
		Subject:     strings.ReplaceAll((*dm.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string), " ", "_"),
		Title:       (*dm.Metadata)["dublin_core"].(map[string]interface{})["title"].(string),
		Version:     fmt.Sprintf("%v", (*dm.Metadata)["dublin_core"].(map[string]interface{})["version"]),
		Owner:       dm.Repo.OwnerName,
		RepoName:    dm.Repo.Name,
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
