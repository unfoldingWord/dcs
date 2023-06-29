// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/modules/log"
)

// GetLatestProdDm gets the latest prod Door43Metadata
func (repo *Repository) GetLatestProdDm() *Door43Metadata {
	if repo.LatestProdDmID > 0 && repo.LatestProdDm == nil {
		dm, err := GetDoor43MetadataByID(repo.LatestProdDmID, repo.ID)
		if err != nil {
			if IsErrDoor43MetadataNotExist(err) {
				log.Warn("Unable to load LatestProdDm for %s: does not exist [%d]", repo.FullName(), repo.LatestProdDmID)
			} else {
				log.Error("GetDoor43MetadataByID Error [%s, %d]: %#v", repo.FullName(), repo.LatestProdDmID, err)
			}
			return nil
		}
		if dm.RepoID == repo.ID && dm.Stage == door43metadata.StageProd {
			dm.Repo = repo
			repo.LatestProdDm = dm
		}
	}
	return repo.LatestProdDm
}

// GetLatestPreprodDm gets the latest preprod Door43Metadata
func (repo *Repository) GetLatestPreprodDm() *Door43Metadata {
	if repo.LatestPreprodDmID > 0 && repo.LatestPreprodDm == nil {
		dm, err := GetDoor43MetadataByID(repo.LatestPreprodDmID, repo.ID)
		if err != nil {
			if IsErrDoor43MetadataNotExist(err) {
				log.Warn("Unable to load LatestPreprodDm for %s: does not exist [%d]", repo.FullName(), repo.LatestPreprodDmID)
			} else {
				log.Error("GetDoor43MetadataByID Error [%s, %d]: %#v", repo.FullName(), repo.LatestPreprodDmID, err)
			}
			return nil
		}
		if dm.RepoID == repo.ID && dm.Stage == door43metadata.StagePreProd {
			dm.Repo = repo
			repo.LatestPreprodDm = dm
		}
	}
	return repo.LatestPreprodDm
}

// GetDefaultBranchDm gets the default branch Door43Metadata
func (repo *Repository) GetDefaultBranchDm() *Door43Metadata {
	if repo.DefaultBranchDmID > 0 && repo.DefaultBranchDm == nil {
		dm, err := GetDoor43MetadataByID(repo.DefaultBranchDmID, repo.ID)
		if err != nil {
			if IsErrDoor43MetadataNotExist(err) {
				log.Warn("Unable to load DefaultBranchDm for %s: does not exist [id: %d]", repo.FullName(), repo.DefaultBranchDmID)
			} else {
				log.Error("GetDoor43MetadataByID Error [%s, %d]: %#v", repo.FullName(), repo.LatestProdDmID, err)
			}
			return nil
		}
		if dm.RepoID == repo.ID && dm.Stage == door43metadata.StageLatest {
			repo.DefaultBranchDm = dm
			dm.Repo = repo
		}
	}
	return repo.DefaultBranchDm
}

// GetRepoDm gets the Door43Metadata that a repo was updated with
func (repo *Repository) GetRepoDm() *Door43Metadata {
	if repo.RepoDmID > 0 && repo.RepoDm == nil {
		dm, err := GetDoor43MetadataByID(repo.RepoDmID, repo.ID)
		if err != nil {
			if IsErrDoor43MetadataNotExist(err) {
				log.Warn("Unable to load RepoDm for %s: does not exist [id: %d]", repo.FullName(), repo.RepoDmID)
			} else {
				log.Error("GetDoor43MetadataByID Error [%s, %d]: %#v", repo.FullName(), repo.RepoDmID, err)
			}
			return nil
		}
		if dm.RepoID == repo.ID {
			dm.Repo = repo
			repo.RepoDm = dm
		}
	}
	return repo.RepoDm
}
