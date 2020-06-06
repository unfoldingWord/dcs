// Copyright 2020 unfolindgWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43Metadata

import (
	"fmt"

	"code.gitea.io/gitea/models"

	"github.com/unknwon/com"
	"xorm.io/xorm"
)

// GenerateDoor43Metadata Generate door43 metadata for valid repos not in the door43_metadata table
func GenerateDoor43Metadata(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

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
		if err = models.ProcessDoor43MetadataForRepoRelease(repo, release); err == nil {
			continue
		}
	}

	return nil
}
