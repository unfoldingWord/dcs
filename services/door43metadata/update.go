// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"

	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/log"
)

// UpdateDoor43Metadata generates door43_metadata table entries for valid repos/releases that don't have them
func UpdateDoor43Metadata(ctx context.Context) error {
	log.Trace("Doing: UpdateDoor43Metadata")

	repos, err := repo.GetReposForMetadata(ctx)
	if err != nil {
		log.Error("GetReposForMetadata: %v", err)
	}

	for _, repo := range repos {
		if err := ProcessDoor43MetadataForRepo(ctx, repo, true); err != nil {
			log.Warn("Failed to process metadata for repo (%v): %v", repo, err)
			if err = system.CreateRepositoryNotice("Failed to process metadata for repository (%s): %v", repo.FullName(), err); err != nil {
				log.Error("ProcessDoor43MetadataForRepo: %v", err)
			}
		}
	}

	log.Trace("Finished: UpdateDoor43Metadata")
	return nil
}
