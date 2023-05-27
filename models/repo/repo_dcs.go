// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"github.com/cloudflare/cfssl/log"
)

// GetLatestProdDm gets the latest prod Door43Metadata
func (repo *Repository) GetLatestProdDm() *Door43Metadata {
	if repo.LatestProdDmID > 0 && repo.LatestProdDm == nil {
		dm, err := GetDoor43MetadataByID(repo.LatestProdDmID)
		if err != nil || dm == nil {
			log.Warning("Unable to load LatestProdDm for %s: %#v", repo.FullName(), err)
			return nil
		}
		dm.Repo = repo
		repo.LatestProdDm = dm
	}
	return repo.LatestProdDm
}

// GetLatestPreprodDm gets the latest preprod Door43Metadata
func (repo *Repository) GetLatestPreprodDm() *Door43Metadata {
	if repo.LatestPreprodDmID > 0 && repo.LatestPreprodDm == nil {
		dm, err := GetDoor43MetadataByID(repo.LatestPreprodDmID)
		if err != nil || dm == nil {
			log.Warning("Unable to load LatestPreprodDm for %s: %#v", repo.FullName(), err)
			return nil
		}
		dm.Repo = repo
		repo.LatestPreprodDm = dm
	}
	return repo.LatestPreprodDm
}

// GetDefaultBranchDm gets the default branch Door43Metadata
func (repo *Repository) GetDefaultBranchDm() *Door43Metadata {
	if repo.DefaultBranchDmID > 0 && repo.DefaultBranchDm == nil {
		dm, err := GetDoor43MetadataByID(repo.DefaultBranchDmID)
		if err != nil || dm == nil {
			log.Warning("Unable to load DefaultBranchDm for %s: %#v", repo.FullName(), err)
			return nil
		}
		repo.DefaultBranchDm = dm
		dm.Repo = repo
	}
	return repo.DefaultBranchDm
}

// GetRepoDm gets the Door43Metadata that a repo was updated with
func (repo *Repository) GetRepoDm() *Door43Metadata {
	if repo.RepoDmID > 0 && repo.RepoDm == nil {
		dm, err := GetDoor43MetadataByID(repo.RepoDmID)
		if err != nil || dm == nil {
			log.Warning("Unable to load RepoDm for %s: %#v", repo.FullName(), err)
			return nil
		}
		dm.Repo = repo
		repo.RepoDm = dm
	}
	return repo.RepoDm
}
