// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git/gitrepo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateDoor43MetadataForOwners re-processes the repos of the given owners only, as
// `gitea door43metadata --owner user5` does.
func TestUpdateDoor43MetadataForOwners(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()

	const branch = "dcs-owner-run"
	for _, id := range []int64{1, 4} { // user2/repo1 and user5/repo4
		repo, err := repo_model.GetRepositoryByID(ctx, id)
		require.NoError(t, err)
		commitFileOnBranch(t, gitrepo.RepoLocalPath(repo.CodeStorageRepo()), branch, "manifest.yaml", validRCManifest)
	}
	require.NoError(t, UpdateDoor43Metadata(ctx, repo_model.ReposForMetadataOptions{OwnerIDs: []int64{5}}))

	dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, 4, branch)
	require.NoError(t, err, "user5's repo is processed")
	assert.Equal(t, "rc", dm.MetadataType)
	_, err = repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, 1, branch)
	assert.True(t, repo_model.IsErrDoor43MetadataNotExist(err), "user2's repo is left alone")
}
