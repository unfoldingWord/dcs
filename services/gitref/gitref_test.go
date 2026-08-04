// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitref

import (
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestGitRef_Get(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repoPath := repo_model.RepoPath(user.Name, repo.Name)

	gitRepo, err := git.OpenRepositoryLocal(repoPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	ref, err := GetReference(t.Context(), gitRepo, "refs/heads/master")
	assert.NoError(t, err)

	assert.NotNil(t, ref)
}

// TestCheckReferenceEditability guards the ref-type parsing: the type is the second
// component ("heads"/"tags"/"pull"), so reading it from the wrong index silently skips
// every protection check below rather than failing loudly.
func TestCheckReferenceEditability(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	const userID = 2 // not on any protection allowlist

	t.Run("malformed names", func(t *testing.T) {
		for _, refName := range []string{"refs/heads", "foo/heads/master"} {
			err := CheckReferenceEditability(t.Context(), refName, "", repo1.ID, userID)
			assert.True(t, git.IsErrInvalidRefName(err), "%s: got %v", refName, err)
		}
	})

	t.Run("refs/pull is read-only", func(t *testing.T) {
		err := CheckReferenceEditability(t.Context(), "refs/pull/42/head", "", repo1.ID, userID)
		assert.True(t, git.IsErrInvalidRefName(err), "got %v", err)
	})

	// repo 1 protects tags matching "v-*" with an empty allowlist
	t.Run("protected tag is rejected", func(t *testing.T) {
		err := CheckReferenceEditability(t.Context(), "refs/tags/v-1.0", "", repo1.ID, userID)
		assert.True(t, git.IsErrProtectedRefName(err), "got %v", err)
	})

	t.Run("unprotected tag is allowed", func(t *testing.T) {
		assert.NoError(t, CheckReferenceEditability(t.Context(), "refs/tags/other-1.0", "", repo1.ID, userID))
	})

	// must run before the subtest below protects master
	t.Run("unprotected branch is allowed", func(t *testing.T) {
		assert.NoError(t, CheckReferenceEditability(t.Context(), "refs/heads/master", "", repo1.ID, userID))
		assert.NoError(t, CheckReferenceEditability(t.Context(), "refs/heads/feature/x", "", repo1.ID, userID))
	})

	t.Run("protected branch is rejected", func(t *testing.T) {
		ctx, committer, err := db.TxContext(t.Context())
		require.NoError(t, err)
		defer committer.Close()
		require.NoError(t, git_model.UpdateProtectBranch(ctx, repo1, &git_model.ProtectedBranch{
			RepoID:   repo1.ID,
			RuleName: "master",
		}, git_model.WhitelistOptions{}))
		require.NoError(t, committer.Commit())

		err = CheckReferenceEditability(t.Context(), "refs/heads/master", "", repo1.ID, userID)
		assert.True(t, git.IsErrProtectedRefName(err), "got %v", err)
	})
}
