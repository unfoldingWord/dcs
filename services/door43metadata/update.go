// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43metadata

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

// UpdateDoor43Metadata generates door43_metadata table entries for valid repos/releases that don't have them
func UpdateDoor43Metadata(ctx context.Context) error {
	log.Trace("Doing: UpdateDoor43Metadata")

	repoIDs, err := repo.GetRepoIDsForMetadata(ctx)
	if err != nil {
		log.Error("GetRepoIDsForMetadata: %v", err)
	}

	if err = db.Iterate(
		ctx,
		new(repo.Repository),
		builder.In("id", repoIDs),
		func(idx int, bean interface{}) error {
			repo := bean.(*repo.Repository)
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before update door43 metadata of %s", repo.FullName())
			default:
			}
			log.Trace("Running generate metadata on %v", repo)
			if err := ProcessDoor43MetadataForRepo(ctx, repo, true); err != nil {
				log.Warn("Failed to process metadata for repo (%v): %v", repo, err)
				if err = system.CreateRepositoryNotice("Failed to process metadata for repository (%s): %v", repo.FullName(), err); err != nil {
					log.Error("ProcessDoor43MetadataForRepo: %v", err)
				}
			}
			return nil
		},
	); err != nil {
		log.Trace("Error: UpdateDoor43Metadata: %v", err)
		return err
	}

	log.Trace("Finished: UpdateDoor43Metadata")
	return nil
}
