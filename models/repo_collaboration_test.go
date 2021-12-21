// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestRepository_AddCollaborator(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(repoID, userID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		assert.NoError(t, repo.GetOwner(db.DefaultContext))
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID}).(*user_model.User)
		assert.NoError(t, AddCollaborator(repo, user))
		unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repoID}, &user_model.User{ID: userID})
	}
	testSuccess(1, 4)
	testSuccess(1, 4)
	testSuccess(3, 4)
}

func TestRepository_GetCollaborators(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(repoID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		collaborators, err := GetCollaborators(repo.ID, db.ListOptions{})
		assert.NoError(t, err)
		expectedLen, err := db.GetEngine(db.DefaultContext).Count(&Collaboration{RepoID: repoID})
		assert.NoError(t, err)
		assert.Len(t, collaborators, int(expectedLen))
		for _, collaborator := range collaborators {
			assert.EqualValues(t, collaborator.User.ID, collaborator.Collaboration.UserID)
			assert.EqualValues(t, repoID, collaborator.Collaboration.RepoID)
		}
	}
	test(1)
	test(2)
	test(3)
	test(4)
}

func TestRepository_IsCollaborator(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(repoID, userID int64, expected bool) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		actual, err := IsCollaborator(repo.ID, userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
	test(3, 2, true)
	test(3, unittest.NonexistentID, false)
	test(4, 2, false)
	test(4, 4, true)
}

func TestRepository_ChangeCollaborationAccessMode(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4}).(*repo_model.Repository)
	assert.NoError(t, ChangeCollaborationAccessMode(repo, 4, perm.AccessModeAdmin))

	collaboration := unittest.AssertExistsAndLoadBean(t, &Collaboration{RepoID: repo.ID, UserID: 4}).(*Collaboration)
	assert.EqualValues(t, perm.AccessModeAdmin, collaboration.Mode)

	access := unittest.AssertExistsAndLoadBean(t, &Access{UserID: 4, RepoID: repo.ID}).(*Access)
	assert.EqualValues(t, perm.AccessModeAdmin, access.Mode)

	assert.NoError(t, ChangeCollaborationAccessMode(repo, 4, perm.AccessModeAdmin))

	assert.NoError(t, ChangeCollaborationAccessMode(repo, unittest.NonexistentID, perm.AccessModeAdmin))

	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repo.ID})
}

func TestRepository_DeleteCollaboration(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4}).(*repo_model.Repository)
	assert.NoError(t, repo.GetOwner(db.DefaultContext))
	assert.NoError(t, DeleteCollaboration(repo, 4))
	unittest.AssertNotExistsBean(t, &Collaboration{RepoID: repo.ID, UserID: 4})

	assert.NoError(t, DeleteCollaboration(repo, 4))
	unittest.AssertNotExistsBean(t, &Collaboration{RepoID: repo.ID, UserID: 4})

	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: repo.ID})
}
