// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package release

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"fmt"
)

// CreateDoor43Metadata creates a new door43 metadata of repository.
func CreateDoor43Metadata(gitRepo *git.Repository, dm *models.Door43Metadata) error {
	isExist, err := models.IsDoor43MetadataExist(dm.RepoID, dm.ReleaseID)
	if err != nil {
		return err
	} else if isExist {
		return models.ErrDoor43MetadataAlreadyExist{
			ReleaseID: dm.ReleaseID,
		}
	}

	if err = models.InsertDoor43Metadata(dm); err != nil {
		return err
	}

	return nil
}

// UpdateDoor43Metadata updates information of a release.
func UpdateDoor43Metadata(doer *models.User, gitRepo *git.Repository, rel *models.Door43Metadata, attachmentUUIDs []string) (err error) {
	if err = models.UpdateDoor43Metadata(models.DefaultDBContext(), rel); err != nil {
		return err
	}

	return err
}

// DeleteDoor43MetadataByID deletes a release and corresponding Git tag by given ID.
func DeleteDoor43MetadataByID(id int64, doer *models.User) error {
	if err := models.DeleteDoor43MetadataByID(id); err != nil {
		return fmt.Errorf("DeleteDoor43MetadataByID: %v", err)
	}

	return nil
}
