// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/santhosh-tekuri/jsonschema/v5"
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

// TestUpdateDoor43MetadataWritesClearedFields checks that re-processing can empty a column:
// an invalid manifest clears what an earlier run stored (#465).
func TestUpdateDoor43MetadataWritesClearedFields(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	dm := &repo_model.Door43Metadata{
		RepoID: 1, Ref: "dcs-cleared", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000465",
		MetadataType: "tc", MetadataVersion: "8", Subject: "Old subject", Flavor: "x-old", Title: "Old title",
		Language: "ru", LanguageIsGL: true, CheckingLevel: 3,
	}
	require.NoError(t, repo_model.InsertDoor43Metadata(ctx, dm))

	dm.Subject, dm.Flavor, dm.Title, dm.Language, dm.LanguageIsGL, dm.CheckingLevel = "", "", "", "", false, 0
	require.NoError(t, repo_model.UpdateDoor43Metadata(ctx, dm))

	stored, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, 1, "dcs-cleared")
	require.NoError(t, err)
	assert.Empty(t, stored.Subject)
	assert.Empty(t, stored.Flavor)
	assert.Empty(t, stored.Title)
	assert.Empty(t, stored.Language)
	assert.False(t, stored.LanguageIsGL)
	assert.Zero(t, stored.CheckingLevel)
}

func TestDoor43MetadataArchiveURLs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	const sha = "65f1bf27bc3bf70f64657658635e66094edbcb4d"

	tag := &repo_model.Door43Metadata{Repo: repo, Ref: "v1.1", RefType: "tag", CommitSHA: sha, MetadataType: "rc", Subject: "Bible"}
	assert.Equal(t, repo.HTMLURL()+"/archive/v1.1.zip", tag.ZipballURL())
	assert.Equal(t, repo.HTMLURL()+"/archive/v1.1.tar.gz", tag.TarballURL())
	assert.Equal(t, repo.HTMLURL()+"/sb/v1.1.zip", tag.SBZipballURL())
	assert.Equal(t, repo.HTMLURL()+"/sb/v1.1.tar.gz", tag.SBTarballURL())
	assert.Equal(t, repo.HTMLURL()+"/sb/raw/metadata.json?ref=v1.1", tag.SBMetadataURL())
	plusTag := &repo_model.Door43Metadata{Repo: repo, Ref: "v1+dcs", RefType: "tag", CommitSHA: sha, MetadataType: "rc", Subject: "Bible"}
	assert.Equal(t, repo.HTMLURL()+"/sb/raw/metadata.json?ref=v1%2Bdcs", plusTag.SBMetadataURL())

	// A branch entry is pinned to the commit it was built from, not the moving branch name
	branch := &repo_model.Door43Metadata{Repo: repo, Ref: "master", RefType: "branch", CommitSHA: sha, MetadataType: "rc", Subject: "Bible"}
	assert.Equal(t, repo.HTMLURL()+"/archive/65f1bf27bc.zip", branch.ZipballURL())
	assert.Equal(t, repo.HTMLURL()+"/archive/65f1bf27bc.tar.gz", branch.TarballURL())
	assert.Equal(t, repo.HTMLURL()+"/sb/65f1bf27bc.zip", branch.SBZipballURL())
	assert.Equal(t, repo.HTMLURL()+"/sb/65f1bf27bc.tar.gz", branch.SBTarballURL())
	assert.Equal(t, repo.HTMLURL()+"/sb/raw/metadata.json?ref=65f1bf27bc", branch.SBMetadataURL())

	lexicon := &repo_model.Door43Metadata{Repo: repo, Ref: "v1", RefType: "tag", CommitSHA: sha, MetadataType: "rc", Subject: "Greek Lexicon"}
	assert.Empty(t, lexicon.SBZipballURL(), "no converter supports the subject, so there is no SB download")
	assert.Empty(t, lexicon.SBMetadataURL())
	assert.Equal(t, repo.HTMLURL()+"/archive/v1.zip", lexicon.ZipballURL())
}

func TestGetReposForMetadataOptions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	ids := func(t *testing.T, opts repo_model.ReposForMetadataOptions) []int64 {
		t.Helper()
		repos, err := repo_model.GetReposForMetadata(ctx, opts)
		require.NoError(t, err)
		var ids []int64
		for _, repo := range repos {
			assert.False(t, repo.IsPrivate || repo.IsArchived, "private and archived repos have no metadata to process")
			ids = append(ids, repo.ID)
		}
		return ids
	}

	all := ids(t, repo_model.ReposForMetadataOptions{})
	owner5 := ids(t, repo_model.ReposForMetadataOptions{OwnerIDs: []int64{5}})
	both := ids(t, repo_model.ReposForMetadataOptions{OwnerIDs: []int64{2, 5}})
	require.NotEmpty(t, owner5)
	assert.Subset(t, both, owner5)
	assert.Contains(t, both, int64(1), "user2/repo1")
	assert.Greater(t, len(all), len(both))
	assert.Equal(t, []int64{1}, ids(t, repo_model.ReposForMetadataOptions{OwnerIDs: []int64{2}, RepoName: "REPO1"}))

	// repo1 has the topics golang and database, repo33 golang only
	assert.Equal(t, []int64{1}, ids(t, repo_model.ReposForMetadataOptions{Topics: []string{"golang", "Database"}}), "a repo needs every topic")
	assert.Subset(t, ids(t, repo_model.ReposForMetadataOptions{Topics: []string{"golang"}}), []int64{1})

	withRelease := ids(t, repo_model.ReposForMetadataOptions{HasRelease: true})
	assert.Contains(t, withRelease, int64(1), "repo1 has published releases")
	assert.NotContains(t, withRelease, int64(4), "repo4 has no releases")

	for _, dm := range []*repo_model.Door43Metadata{
		{RepoID: 1, Ref: "dcs-error", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000001", MetadataType: "rc", HealthcheckSeverity: repo_model.SeverityLevelError},
		{RepoID: 4, Ref: "dcs-warning", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000004", MetadataType: "rc", HealthcheckSeverity: repo_model.SeverityLevelWarning},
		{RepoID: 4, Ref: "dcs-invalid", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000044", MetadataType: "tc", ValidationError: &jsonschema.ValidationError{Message: "bad manifest"}},
	} {
		require.NoError(t, repo_model.InsertDoor43Metadata(ctx, dm))
	}
	assert.Equal(t, []int64{1}, ids(t, repo_model.ReposForMetadataOptions{MinHealthcheckSeverity: repo_model.SeverityLevelError}))
	assert.ElementsMatch(t, []int64{1, 4}, ids(t, repo_model.ReposForMetadataOptions{MinHealthcheckSeverity: repo_model.SeverityLevelWarning}))
	assert.Equal(t, []int64{4}, ids(t, repo_model.ReposForMetadataOptions{HasValidationError: true}))
	assert.Empty(t, ids(t, repo_model.ReposForMetadataOptions{HasValidationError: true, HasRelease: true}), "every option must hold")
	assert.Equal(t, []int64{1}, ids(t, repo_model.ReposForMetadataOptions{MinHealthcheckSeverity: repo_model.SeverityLevelWarning, HasRelease: true}))

	release := func(tag string, draft, bareTag bool) *repo_model.Release {
		return &repo_model.Release{RepoID: 4, PublisherID: 5, TagName: tag, LowerTagName: tag, Target: "master", Title: tag, IsDraft: draft, IsTag: bareTag}
	}
	require.NoError(t, db.Insert(ctx, release("v-draft", true, false)))
	require.NoError(t, db.Insert(ctx, release("v-tag", false, true)))
	assert.NotContains(t, ids(t, repo_model.ReposForMetadataOptions{HasRelease: true}), int64(4), "a draft and a bare tag are no release")
	require.NoError(t, db.Insert(ctx, release("v1", false, false)))
	assert.Contains(t, ids(t, repo_model.ReposForMetadataOptions{HasRelease: true}), int64(4), "a published release is")

	_, err := repo_model.GetReposForMetadata(ctx, repo_model.ReposForMetadataOptions{RepoName: "repo1"})
	assert.Error(t, err, "a repo name alone would match that repo of every owner")
}
