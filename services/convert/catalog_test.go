// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	"code.gitea.io/gitea/models/door43metadata"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToCatalogEntryAttachmentTypes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, repo1.LoadOwner(t.Context()))

	// A release entry gets an attachment_types object, even when all flags are false
	dm := &repo_model.Door43Metadata{
		RepoID: 1, Repo: repo1, ReleaseID: 1002, Ref: "v9", RefType: "tag",
		CommitSHA: "0000000000000000000000000000000000000009",
		Stage:     door43metadata.StageProd,
		HasPDF:    true, HasAudio: true,
		ReleaseDateUnix: timeutil.TimeStamp(2000),
	}
	entry := ToCatalogEntry(t.Context(), dm, nil, nil)
	require.NotNil(t, entry)
	require.NotNil(t, entry.AttachmentTypes)
	assert.True(t, entry.AttachmentTypes.PDF)
	assert.True(t, entry.AttachmentTypes.Audio)
	assert.False(t, entry.AttachmentTypes.Video)
	assert.False(t, entry.AttachmentTypes.Stream)
	assert.False(t, entry.AttachmentTypes.Other)

	// A branch entry (no release) gets null
	dm.ReleaseID = 0
	dm.Ref = "master"
	dm.RefType = "branch"
	entry = ToCatalogEntry(t.Context(), dm, nil, nil)
	require.NotNil(t, entry)
	assert.Nil(t, entry.AttachmentTypes)
}
