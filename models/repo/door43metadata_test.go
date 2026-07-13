// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetermineAttachmentFlags(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Release 1 has one fixture attachment, "attach1", which matches no
	// content type, so it counts as "other".
	dm := &repo_model.Door43Metadata{RepoID: 1, ReleaseID: 1}
	require.NoError(t, dm.DetermineAttachmentFlags(t.Context()))
	assert.False(t, dm.HasAudio)
	assert.False(t, dm.HasVideo)
	assert.False(t, dm.HasPDF)
	assert.False(t, dm.HasStream)
	assert.True(t, dm.HasOther)

	// A release with one attachment of each content type, plus a files.json
	// manifest which must be skipped (it would otherwise count as "other").
	const releaseID int64 = 9999
	attachments := []*repo_model.Attachment{
		{UUID: "dcs-flags-test-audio", ReleaseID: releaseID, Name: "fr_obs_v4.3_mp3_128kbps.zip"},
		{UUID: "dcs-flags-test-video", ReleaseID: releaseID, Name: "en_obs_v6_01_720p.mp4"},
		{UUID: "dcs-flags-test-pdf", ReleaseID: releaseID, Name: "en_obs_v9.pdf"},
		{UUID: "dcs-flags-test-stream", ReleaseID: releaseID, Name: "YouTube", BrowserDownloadURL: "https://www.youtube.com/playlist?list=PLdE_o99zEY27D4H6XdxmN9ZmUluJKZZTo"},
		{UUID: "dcs-flags-test-manifest", ReleaseID: releaseID, Name: "files.json"},
	}
	for _, attachment := range attachments {
		_, err := db.GetEngine(t.Context()).Insert(attachment)
		require.NoError(t, err)
	}
	dm = &repo_model.Door43Metadata{RepoID: 1, ReleaseID: releaseID}
	require.NoError(t, dm.DetermineAttachmentFlags(t.Context()))
	assert.True(t, dm.HasAudio)
	assert.True(t, dm.HasVideo)
	assert.True(t, dm.HasPDF)
	assert.True(t, dm.HasStream)
	assert.False(t, dm.HasOther)

	// A DM without a release (branch ref) always has all flags reset.
	dm = &repo_model.Door43Metadata{RepoID: 1, HasAudio: true, HasVideo: true, HasPDF: true, HasStream: true, HasOther: true}
	require.NoError(t, dm.DetermineAttachmentFlags(t.Context()))
	assert.False(t, dm.HasAudio)
	assert.False(t, dm.HasVideo)
	assert.False(t, dm.HasPDF)
	assert.False(t, dm.HasStream)
	assert.False(t, dm.HasOther)
}
