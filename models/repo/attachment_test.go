// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
)

func TestIncreaseDownloadCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	attachment, err := repo_model.GetAttachmentByUUID(t.Context(), "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), attachment.DownloadCount)

	// increase download count
	err = attachment.IncreaseDownloadCount(t.Context())
	assert.NoError(t, err)

	attachment, err = repo_model.GetAttachmentByUUID(t.Context(), "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), attachment.DownloadCount)
}

func TestGetByCommentOrIssueID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// count of attachments from issue ID
	attachments, err := repo_model.GetAttachmentsByIssueID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Len(t, attachments, 1)

	attachments, err = repo_model.GetAttachmentsByCommentID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Len(t, attachments, 2)
}

func TestDeleteAttachments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	count, err := repo_model.DeleteAttachmentsByIssue(t.Context(), 4, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	count, err = repo_model.DeleteAttachmentsByComment(t.Context(), 2, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	err = repo_model.DeleteAttachment(t.Context(), &repo_model.Attachment{ID: 8}, false)
	assert.NoError(t, err)

	attachment, err := repo_model.GetAttachmentByUUID(t.Context(), "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a18")
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrAttachmentNotExist(err))
	assert.Nil(t, attachment)
}

func TestGetAttachmentByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	attach, err := repo_model.GetAttachmentByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.UUID)
}

func TestAttachment_DownloadURL(t *testing.T) {
	attach := &repo_model.Attachment{
		UUID: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
		ID:   1,
	}
	assert.Equal(t, "https://try.gitea.io/attachments/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.DownloadURL())
}

func TestUpdateAttachment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	attach, err := repo_model.GetAttachmentByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.UUID)

	attach.Name = "new_name"
	assert.NoError(t, repo_model.UpdateAttachment(t.Context(), attach))

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{Name: "new_name"})
}

func TestGetAttachmentsByUUIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	attachList, err := repo_model.GetAttachmentsByUUIDs(t.Context(), []string{"a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a17", "not-existing-uuid"})
	assert.NoError(t, err)
	assert.Len(t, attachList, 2)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attachList[0].UUID)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a17", attachList[1].UUID)
	assert.Equal(t, int64(1), attachList[0].IssueID)
	assert.Equal(t, int64(5), attachList[1].IssueID)
}

/*** DCS Customizations ***/

// DCS stores "remote" release attachments (links to YouTube, cloud storage,
// etc. that are not uploaded blobs) by encoding the external URL into the Name
// column as "name|url". BeforeInsert/BeforeUpdate write that encoding and
// AfterLoad/AfterInsert/AfterUpdate split it back into Name + BrowserDownloadURL.
// These are the building blocks of the files.json / links.json manifest feature
// expanded by services/door43metadata.UnpackJSONAttachments.

// TestAttachment_ExternalURLEncoding verifies the happy path used by a
// files.json entry that supplies both a name and a browser_download_url
// (e.g. {"name": "YouTube - OBS - Awadhi", "browser_download_url": "https://..."}).
// The name must survive the insert encoding and the subsequent load.
func TestAttachment_ExternalURLEncoding(t *testing.T) {
	const name = "YouTube - OBS - Awadhi"
	const link = "https://www.youtube.com/playlist?list=PLcaGjtXr4D9kq36LRMtXLoFZ3eB9m4Drd"

	attach := &repo_model.Attachment{Name: name, BrowserDownloadURL: link}

	// BeforeInsert encodes the URL into the Name column as "name|url".
	attach.BeforeInsert()
	assert.Equal(t, name+"|"+link, attach.Name)

	// AfterLoad (run on read, and via AfterInsert) splits it back out.
	attach.AfterLoad()
	assert.Equal(t, name, attach.Name)
	assert.Equal(t, link, attach.BrowserDownloadURL)

	// DownloadURL points at the external link rather than a stored blob.
	assert.Equal(t, link, attach.DownloadURL())
}

// TestAttachment_ExternalURLFallbackName documents why a manifest entry that
// omits "name" ends up displayed as "playlist": BeforeInsert falls back to
// path.Base of the URL path, and a YouTube playlist URL has the path
// "/playlist" (the real identifier lives in the ?list= query string). This is
// the root cause of the qa.door43.org awa_obs release showing "playlist".
func TestAttachment_ExternalURLFallbackName(t *testing.T) {
	const link = "https://www.youtube.com/playlist?list=PLcaGjtXr4D9kq36LRMtXLoFZ3eB9m4Drd"

	attach := &repo_model.Attachment{BrowserDownloadURL: link} // no Name supplied
	attach.BeforeInsert()
	attach.AfterLoad()
	assert.Equal(t, "playlist", attach.Name, "an entry without a name falls back to path.Base of the URL path")
	assert.Equal(t, link, attach.BrowserDownloadURL)
}

// TestAttachment_ManifestUnmarshal verifies that the files.json / links.json
// manifest shape parses into the fields the unpack logic relies on, using the
// real modules/json unmarshaler. Under GOEXPERIMENT=jsonv2 that matcher is
// case-sensitive, so this guards the explicit json:"name"/json:"size" tags on
// the Attachment struct: drop them and Name/Size stop binding (the root cause
// of the "playlist" name on the awa_obs release). This is the exact shape that
// was uploaded for that release.
func TestAttachment_ManifestUnmarshal(t *testing.T) {
	const manifest = `[
  {
    "name": "YouTube - OBS - Awadhi",
    "size": 0,
    "browser_download_url": "https://www.youtube.com/playlist?list=PLcaGjtXr4D9kq36LRMtXLoFZ3eB9m4Drd"
  }
]`

	attachments := []*repo_model.Attachment{}
	assert.NoError(t, json.Unmarshal([]byte(manifest), &attachments))
	assert.Len(t, attachments, 1)
	assert.Equal(t, "YouTube - OBS - Awadhi", attachments[0].Name)
	assert.Equal(t, "https://www.youtube.com/playlist?list=PLcaGjtXr4D9kq36LRMtXLoFZ3eB9m4Drd", attachments[0].BrowserDownloadURL)
	assert.Equal(t, int64(0), attachments[0].Size)
}

/*** END DCS Customizations ***/

func TestGetUnlinkedAttachmentsByUserID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	attachments, err := repo_model.GetUnlinkedAttachmentsByUserID(t.Context(), 8)
	assert.NoError(t, err)
	assert.Len(t, attachments, 1)
	assert.Equal(t, int64(10), attachments[0].ID)
	assert.Zero(t, attachments[0].IssueID)
	assert.Zero(t, attachments[0].ReleaseID)
	assert.Zero(t, attachments[0].CommentID)

	attachments, err = repo_model.GetUnlinkedAttachmentsByUserID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Empty(t, attachments)
}
