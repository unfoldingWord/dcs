// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitref

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestGitRef_Get(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repoPath := repo_model.RepoPath(user.Name, repo.Name)

	gitRepo, err := git.OpenRepository(repoPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	ref, err := GetReference(t.Context(), gitRepo, "refs/heads/master")
	assert.NoError(t, err)

	assert.NotNil(t, ref)
}
