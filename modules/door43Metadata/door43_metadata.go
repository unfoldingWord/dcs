// Copyright 2020 unfolindgWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43Metadata

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/unknwon/com"
	"io/ioutil"

	"github.com/xeipuuv/gojsonschema"
	"xorm.io/xorm"
)

var rc02Schema []byte

// GetRC02Schema Returns the schema for RC v0.2, retrieving it from file if not already done
func GetRC02Schema() ([]byte, error) {
	if rc02Schema == nil {
		var err error
		rc02Schema, err = models.GetRepoInitFile("schema", "rc.schema.json")
		if err != nil {
			return nil, err
		}
	}
	return rc02Schema, nil
}

func generateDoor43MetadataForRepoRelease(repo *models.Repository, release *models.Release) error {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		fmt.Printf("OpenRepository Error: %v\n", err)
		return err
	}
	defer gitRepo.Close()

	var commit *git.Commit
	if release == nil {
		commit, err = gitRepo.GetBranchCommit(repo.DefaultBranch)
		if err != nil {
			fmt.Printf("GetBranchCommit Error: %v\n", err)
			return err
		}
	} else {
		commit, err = gitRepo.GetTagCommit(release.TagName)
		if err != nil {
			fmt.Printf("GetTagCommit Error: %v\n", err)
			return err
		}
	}

	entry, err := commit.GetTreeEntryByPath("manifest.yaml")
	if err != nil {
		fmt.Printf("GetTreeEntryByPath Error: %v\n", err)
		return err
	}
	dataRc, err := entry.Blob().DataAsync()
	if err != nil {
		fmt.Printf("DataAsync Error: %v\n", err)
		return err
	}
	defer dataRc.Close()

	content, _ := ioutil.ReadAll(dataRc)
	//fmt.Printf("content: %s", content)

	var manifest map[string]interface{}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		fmt.Printf("yaml.Unmarshal Error: %v", err)
		return err
	}

	schema, err := GetRC02Schema()
	if err != nil {
		return err
	}
	schemaLoader := gojsonschema.NewBytesLoader(schema)
	documentLoader := gojsonschema.NewGoLoader(manifest)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return err
	}

	var releaseID int64
	if release != nil {
		releaseID = release.ID
	}

	if result.Valid() {
		fmt.Printf("The document is valid\n")
		dm := &models.Door43Metadata{
			RepoID:          repo.ID,
			ReleaseID:       releaseID,
			MetadataVersion: "rc0.2",
			Metadata:        manifest,
		}

		if err = models.InsertDoor43Metadata(dm); err != nil {
			fmt.Printf("Error: %v", err)
			return err
		}

		return nil
	} else {
		fmt.Printf("==========\nREPO: %s\n", repo.Name)
		fmt.Printf("REPO ID: %d, RELEASE ID: %d\n", repo.ID, releaseID)
		fmt.Printf("The document is not valid. see errors :\n")
		if release != nil {
			fmt.Printf("RELEASE: %v\n", release.TagName)
		} else {
			fmt.Printf("BRANCH: %s\n", repo.DefaultBranch)
		}
		for _, desc := range result.Errors() {
			fmt.Printf("- %s\n", desc.Description())
			fmt.Printf("- %s = %s\n", desc.Field(), desc.Value())
		}
		return nil
	}
}

// GenerateDoor43Metadata Generate door43 metadata for valid repos not in the door43_metadata table
func GenerateDoor43Metadata(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	reposWithoutMetadata := []*models.Repository{}

	// Query to find repos that need processing, either having releases that
	// haven't been processed, or their default branch hasn't been processed.
	records, err := sess.Query("SELECT rel.id as release_id, r.id as repo_id  FROM `repository` r " +
		"  JOIN `release` rel ON rel.repo_id = r.id " +
		"  LEFT JOIN `door43_metadata` dm ON r.id = dm.repo_id " +
		"  AND rel.id = dm.release_id " +
		"  WHERE dm.id IS NULL " +
		"UNION " +
		"SELECT 0 as `release_id`, r2.id as repo_id FROM `repository` r2 " +
		"  LEFT JOIN `door43_metadata` dm2 ON r2.id = dm2.repo_id " +
		"  AND dm2.release_id = 0 " +
		"  WHERE dm2.id IS NULL " +
		"ORDER BY repo_id ASC, release_id ASC")
	if err != nil {
		return err
	}

	cacheRepos := make(map[int64]*models.Repository)

	for _, record := range records {
		releaseID := com.StrTo(record["release_id"]).MustInt64()
		repoID := com.StrTo(record["repo_id"]).MustInt64()
		fmt.Printf("HERE ====> Repo: %d, Release: %d\n", repoID, releaseID)
		if cacheRepos[repoID] == nil {
			cacheRepos[repoID], err = models.GetRepositoryByID(repoID)
			if err != nil {
				fmt.Printf("GetRepositoryByID Error: %v\n", err)
				continue
			}
		}
		repo := cacheRepos[repoID]
		var release *models.Release
		if releaseID > 0 {
			release, err = models.GetReleaseByID(releaseID)
			if err != nil {
				fmt.Printf("GetReleaseByID Error: %v\n", err)
				continue
			}
		}
		if err = generateDoor43MetadataForRepoRelease(repo, release); err == nil {
			continue
		}
	}

	fmt.Printf("%v", reposWithoutMetadata)

	return nil
}
