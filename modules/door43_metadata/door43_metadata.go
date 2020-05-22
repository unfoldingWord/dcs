// Copyright 2020 unfolindgWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43_metadata

import (
	"fmt"
	"github.com/unknwon/com"
	"gopkg.in/yaml.v2"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
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
		"ORDER BY id ASC")
	if err != nil {
		return err
	}

	cacheRepos := make(map[int64]*models.Repository)

	for _, record := range records {
		release_id := com.StrTo(record["release_id"]).MustInt64()
		repo_id := com.StrTo(record["repo_id"]).MustInt64()
		if cacheRepos[repo_id] == nil {
			cacheRepos[repo_id], err = models.GetRepositoryByID(repo_id)
			if err != nil {
				return err
			}
		}
		repo := cacheRepos[repo_id]
		fmt.Printf("ID: %d, %d", repo_id, release_id)
		var release *models.Release
		if release_id > 0 {
			release, err = models.GetReleaseByID(release_id)
			if err != nil {
				return err
			}
			fmt.Printf("%v", release)
		}

		gitRepo, err := git.OpenRepository(repo.RepoPath())
		if err != nil {
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
			return err
		}

		entry, err := commit.GetTreeEntryByPath("manifest.yaml")
		if err != nil {
			continue
		}

		gitBlob, err := gitRepo.GetBlob(entry.ID.String())
		if err != nil {
			return err
		}
		content, err := gitBlob.GetBlobContent()
		if err != nil {
			return err
		}
		fmt.Printf("content: %s", content)

		var manifest map[string]interface{}
		if err := yaml.Unmarshal([]byte(content), &manifest); err != nil {
			return err
		}
		fmt.Printf("manifest: %v", manifest)

		if
	}

	fmt.Printf("%v", reposWithoutMetadata)

	return nil
}
