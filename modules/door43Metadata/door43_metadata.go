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

	schema, err := models.GetRepoInitFile("schema", "rc.schema.json")
	if err != nil {
		return err
	}
	schemaLoader := gojsonschema.NewBytesLoader(schema)

	for _, record := range records {
		releaseID := com.StrTo(record["releaseID"]).MustInt64()
		repoID := com.StrTo(record["repoID"]).MustInt64()
		if cacheRepos[repoID] == nil {
			cacheRepos[repoID], err = models.GetRepositoryByID(repoID)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return err
			}
		}
		repo := cacheRepos[repoID]
		var release *models.Release
		if releaseID > 0 {
			release, err = models.GetReleaseByID(releaseID)
			if err != nil {
				return err
			}
		}

		gitRepo, err := git.OpenRepository(repo.RepoPath())
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return err
		}
		defer gitRepo.Close()

		var commit *git.Commit
		if release == nil {
			commit, err = gitRepo.GetBranchCommit("master")
		} else {
			commit, err = gitRepo.GetTagCommit(release.TagName)
		}
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return err
		}

		entry, err := commit.GetTreeEntryByPath("manifest.yaml")
		if err != nil {
			continue
		}
		dataRc, err := entry.Blob().DataAsync()
		if err != nil {
			return err
		}
		defer dataRc.Close()
		content, _ := ioutil.ReadAll(dataRc)
		//fmt.Printf("content: %s", content)

		var manifest map[string]interface{}
		if err := yaml.Unmarshal(content, &manifest); err != nil {
			fmt.Printf("Error: %v", err)
			return err
		}

		documentLoader := gojsonschema.NewGoLoader(manifest)

		result, err := gojsonschema.Validate(schemaLoader, documentLoader)
		if err != nil {
			fmt.Printf("Error: %v", err)
			continue
		}

		if result.Valid() {
			fmt.Printf("The document is valid\n")
			dm := &models.Door43Metadata{
				RepoID:          repoID,
				ReleaseID:       releaseID,
				MetadataVersion: "rc0.2",
				Metadata:        manifest,
			}

			if err = models.InsertDoor43Metadata(dm); err != nil {
				fmt.Printf("Error: %v", err)
				continue
			}
		} else {
			fmt.Printf("==========\nREPO: %s\n", repo.Name)
			fmt.Printf("REPO ID: %d, RELEASE ID: %d\n", repoID, releaseID)
			fmt.Printf("The document is not valid. see errors :\n")
			if releaseID > 0 {
				fmt.Printf("RELEASE: %v\n", release.TagName)
			} else {
				fmt.Printf("BRANCH: %s\n", repo.DefaultBranch)
			}
			for _, desc := range result.Errors() {
				fmt.Printf("- %s\n", desc.Description())
				fmt.Printf("- %s = %s\n", desc.Field(), desc.Value())
			}
			continue
		}
	}

	fmt.Printf("%v", reposWithoutMetadata)

	return nil
}
