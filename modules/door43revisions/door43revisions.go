// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43revisions

import (
	"fmt"
	"reflect"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/mitchellh/mapstructure"
	"github.com/unknwon/com"
	"xorm.io/xorm"
)

// GenerateDoor43Revisions Generate door43 revisions for valid repos not in the door43_revision table
func GenerateDoor43Revisions(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	// Query to find repos that need processing, either having releases that
	// haven't been processed, or their default branch hasn't been processed.
	records, err := sess.Query("SELECT r.id as repo_id, rel.id as release_id FROM `repository` r " +
		"  JOIN `release` rel ON rel.repo_id = r.id " +
		"  LEFT JOIN `door43_revision` dr ON r.id = dr.repo_id " +
		"  AND rel.id = dr.release_id " +
		"  WHERE dr.id IS NULL AND rel.is_tag = 0 AND rel.is_prerelease = 0 AND rel.is_draft = 0" +
		"ORDER BY repo_id ASC, release_id ASC")
	if err != nil {
		return err
	}

	cacheRepos := make(map[int64]*models.Repository)

	for _, record := range records {
		repoID := com.StrTo(record["repo_id"]).MustInt64()
		releaseID := com.StrTo(record["release_id"]).MustInt64()
		tagName := record["tag_name"]
		if cacheRepos[repoID] == nil {
			cacheRepos[repoID], err = models.GetRepositoryByID(repoID)
			if err != nil {
				log.Warn("GetRepositoryByID Error: %v\n", err)
				continue
			}
		}
		repo := cacheRepos[repoID]
		var release *models.Release
		if releaseID > 0 {
			release, err = models.GetReleaseByID(releaseID)
			if err != nil {
				log.Warn("GetReleaseByID Error: %v\n", err)
				continue
			}
		}
		if err = ProcessDoor43RevisionForRepoRelease(repo, release); err != nil {
			continue
		}
	}

	return nil
}

// ConvertGenericMapToRC020Manifest converts a generic map to a RC020Manifest object
func ConvertGenericMapToRC020Manifest(manifest *map[string]interface{}) (*structs.RC020Manifest, error) {
	var rc020manifest structs.RC020Manifest
	err := mapstructure.Decode(*manifest, &rc020manifest)
	if err != nil {
		return nil, err
	}

	type Checking struct {
		CheckingLevel string `mapstructure:"checking_level"`
	}
	type Language struct {
		Identifier    string
		LangDirection string `mapstructure:"lang_direction"`
	}
	type DublinCore struct {
		Subject  string
		Language Language
		TestThis string `mapstructure:"test_this"`
	}
	type Project struct {
		Identifier string
	}
	type Person struct {
		Checking
		DublinCore `mapstructure:"dublin_core"`
		Projects   []Project
	}

	book1 := map[string]interface{}{"identifier": "gen"}
	book2 := map[string]interface{}{"identifier": "exo"}
	input := map[string]interface{}{
		"dublin_core": map[string]interface{}{
			"subject": "test",
			"language": map[string]interface{}{
				"identifier":     "en",
				"lang_direction": "ltr",
			},
			"test_this": "ok",
		},
		"checking": map[string]interface{}{"checking_level": "1"},
		"projects": []map[string]interface{}{book1, book2},
	}

	var result Person
	err = mapstructure.Decode(input, &result)
	if err != nil {
		panic(err)
	}

	return &rc020manifest, err
}

// ProcessDoor43RevisionsForRepo handles the metadata for a given repo for all its releases
func ProcessDoor43RevisionsForRepo(repo *models.Repository) error {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}

	if repo.IsArchived || repo.IsPrivate {
		_, err := models.DeleteAllDoor43MetadatasByRepoID(repo.ID)
		if err != nil {
			log.Error("DeleteAllDoor43MetadatasByRepoID: %v", err)
		}
		return err
	}

	relIDs, err := models.GetRepoReleaseIDsForMetadata(repo.ID)
	if err != nil {
		log.Error("GetReleaseIDsNeedingDoor43Metadata: %v", err)
		return err
	}

	for _, releaseID := range relIDs {
		var release *models.Release
		releaseRef := repo.DefaultBranch
		if releaseID > 0 {
			release, err = models.GetReleaseByID(releaseID)
			if err != nil {
				fmt.Printf("GetReleaseByID Error: %v\n", err)
				continue
			}
			releaseRef = release.TagName
		}
		log.Info("Processing Metadata for repo %s (%d), %s (%d)\n", repo.Name, repo.ID, releaseRef, releaseID)
		if err = ProcessDoor43MetadataForRepoRelease(repo, release); err != nil {
			log.Warn("Error processing metadata for repo %s (%d), %s (%d): %v\n", repo.Name, repo.ID, releaseRef, releaseID, err)
		} else {
			log.Info("Processed Metadata for repo %s (%d), %s (%d)\n", repo.Name, repo.ID, releaseRef, releaseID)
		}
	}
	return nil
}

// ProcessDoor43RevisionForRepoRelease handles the revisions for a given repo by release based on if the container is a valid RC or not
func ProcessDoor43RevisionForRepoRelease(repo *models.Repository, release *models.Release, tagName string) error {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}
	if release == nil {
		return fmt.Errorf("no release provided")
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		fmt.Printf("OpenRepository Error: %v\n", err)
		return err
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetTagCommit(release.TagName)
	if err != nil {
		log.Error("GetTagCommit: %v\n", err)
		return err
	}

	blob, err := commit.GetBlobByPath("manifest.yaml")
	if err != nil && !git.IsErrNotExist(err) {
		return err
	}
	if blob == nil {
		return nil
	}

	manifest, err := base.ReadYAMLFromBlob(blob)
	if err != nil {
		return err
	}

	result, err := base.ValidateBlobByRC020Schema(manifest)
	if err != nil {
		return err
	}

	dr, err := models.GetDoor43RevisionByRepoIDAndReleaseID(repo.ID, release.ID)
	if err != nil && !models.IsErrDoor43RevisionNotExist(err) {
		return err
	}

	if dr == nil ||
		release.CreatedUnix != dr.ReleaseDateUnix ||
		dr.TagName != release.TagName) {
		if !result.Valid() {
			log.Warn("%s/%s: manifest.yaml is not valid. see errors:", repo.FullName(), branchOrTag)
			log.Warn("REPO ID: %d, RELEASE ID: %d, RELEASE: %v", repo.ID, release.ID, release.TagName)
			for _, desc := range result.Errors() {
				log.Warn("- %s", desc.Description())
				log.Warn("- %s = %s", desc.Field(), desc.Value())
			}
			if dr != nil {
				return models.DeleteDoor43Revision(dm)
			}
		} else {
			log.Warn("%s/%s: manifest.yaml is valid.", repo.FullName(), release.TagName)
			gitRepo, err := git.OpenRepository(repo.RepoPath())
			if err != nil {
				return err
			}
			defer gitRepo.Close()
			gitTree, err := gitRepo.GetTree(release.TagName)
			if err != nil || gitTree == nil {
				return models.ErrSHANotFound{
					SHA: release.TagName,
				}
			}
			entries, err = gitTree.ListEntriesRecursive()
			if err != nil {
				return err
			}
			for _, entry := range entries:
				path := entry.Name()
				mode := fmt.Sprintf("%06o", entry.Mode())
				type := entry.Type()
				size = entry.Size()
				sha = entry.ID.String()
	
			if entries[e].IsDir() {
				copy(treeURL[copyPos:], entries[e].ID.String())
				tree.Entries[i].URL = string(treeURL)
			} else {
				copy(blobURL[copyPos:], entries[e].ID.String())
				tree.Entries[i].URL = string(blobURL)
			}
			if dr == nil {
				dr = &models.Door43Revision{
					RepoID:          repo.ID,
					Repo:            repo,
					ReleaseID:       releaseID,
					Release:         release,
					ReleaseDateUnix: release.CreatedUnix,
					TagName:     	 release.TagName,
				}
				return models.InsertDoor43Revision(dm)
			}
			dm.Metadata = manifest
			dm.ReleaseDateUnix = releaseDateUnix
			dm.Stage = stage
			dm.BranchOrTag = branchOrTag
			return models.UpdateDoor43MetadataCols(dm, "metadata", "release_date_unix", "stage", "branch_or_tag")
		}
	}

	return nil
}
