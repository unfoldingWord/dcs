// Copyright 2020 unfolindgWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43_metadata

import (
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/unknwon/com"
	"io/ioutil"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"

	"github.com/xeipuuv/gojsonschema"
	"xorm.io/xorm"
)

type RC020 struct {
	ID            int                    `json:"id"`
	Name          string                 `json:"name" jsonschema:"title=the name,description=The name of a friend,example=joe,example=lucy,default=alex"`
	Friends       []int                  `json:"friends,omitempty" jsonschema_description:"The list of IDs, omitted when empty"`
	Tags          map[string]interface{} `json:"tags,omitempty" jsonschema_extras:"a=b,foo=bar,foo=bar1"`
	BirthDate     time.Time              `json:"birth_date,omitempty" jsonschema:"oneof_required=date"`
	YearOfBirth   string                 `json:"year_of_birth,omitempty" jsonschema:"oneof_required=year"`
	Metadata      interface{}            `json:"metadata,omitempty" jsonschema:"oneof_type=string;array"`
	FavColor      string                 `json:"fav_color,omitempty" jsonschema:"enum=red,enum=green,enum=blue"`
}

// Generate door43 metadata for valid repos not in the door43_metadata table
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
		release_id := com.StrTo(record["release_id"]).MustInt64()
		repo_id := com.StrTo(record["repo_id"]).MustInt64()
		if cacheRepos[repo_id] == nil {
			cacheRepos[repo_id], err = models.GetRepositoryByID(repo_id)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return err
			}
		}
		repo := cacheRepos[repo_id]
		var release *models.Release
		if release_id > 0 {
			release, err = models.GetReleaseByID(release_id)
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
				RepoID:    repo_id,
				ReleaseID: release_id,
				MetadataVersion: "rc0.2",
				Metadata: manifest,
			}

			if err = models.InsertDoor43Metadata(dm); err != nil {
				fmt.Printf("Error: %v", err)
				continue
			}
		} else {
			fmt.Printf("==========\nREPO: %s\n", repo.Name)
			fmt.Printf("REPO ID: %d, RELEASE ID: %d\n", repo_id, release_id)
			fmt.Printf("The document is not valid. see errors :\n")
			if release_id > 0 {
				fmt.Printf("RELEASE: %v\n", release.TagName)
			} else {
				fmt.Printf("BRANCH: %s\n", repo.DefaultBranch)
			}
			for _, desc := range result.Errors() {
				fmt.Printf("- %s\n", desc.Description())
				fmt.Printf("- %s = %s\n", desc.Field(), desc.Value())
			}
			continue;
		}
	}

	fmt.Printf("%v", reposWithoutMetadata)

	return nil
}
