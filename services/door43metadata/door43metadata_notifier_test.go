// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"testing"

	repo_model "gitea.dev/models/repo"

	"github.com/stretchr/testify/assert"
)

func TestMetadataQueueItemApplyRename(t *testing.T) {
	rename := metadataQueueItem{RepoID: 1, RenamedFrom: "en_tn", RenamedTo: "en_TN_new"}
	for _, c := range []struct {
		dbName, want string
	}{
		{"en_tn", "en_TN_new"},         // the caller hasn't saved the new name yet
		{"en_TN_new", "en_TN_new"},     // it has
		{"en_tn_later", "en_tn_later"}, // a later rename was saved since
	} {
		repo := &repo_model.Repository{Name: c.dbName}
		rename.applyRename(repo)
		assert.Equal(t, c.want, repo.Name, c.dbName)
	}

	repo := &repo_model.Repository{Name: "en_tn", LowerName: "en_tn"}
	rename.applyRename(repo)
	assert.Equal(t, "en_tn_new", repo.LowerName)

	repo = &repo_model.Repository{Name: "en_tn"}
	metadataQueueItem{RepoID: 1}.applyRename(repo)
	assert.Equal(t, "en_tn", repo.Name, "not a rename")
}
