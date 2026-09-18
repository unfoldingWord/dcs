// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sbarchiver

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"strings"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/services/contexttest"

	_ "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

// repo49 (user27/repo49) has branches "master" and "test/archive"; 51f84af23134 is its initial commit.
const (
	testRepoID      int64 = 49
	testFirstCommit       = "51f84af23134"
)

func insertDM(t *testing.T, dm *repo_model.Door43Metadata) {
	_, err := db.GetEngine(t.Context()).Insert(dm)
	require.NoError(t, err)
}

func loadRepo(t *testing.T) *repo_model.Repository {
	return unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: testRepoID})
}

func TestNewRequest(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx, _ := contexttest.MockContext(t, "user27/repo49")
	contexttest.LoadRepo(t, ctx, testRepoID)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	aReq, err := NewRequest(testRepoID, ctx.Repo.GitRepo, "master.zip")
	require.NoError(t, err)
	assert.Equal(t, repo_model.ArchiveZip, aReq.Type)
	assert.Equal(t, "master-sb.zip", aReq.GetArchiveName())
	masterID, err := ctx.Repo.GitRepo.ConvertToGitID("master")
	require.NoError(t, err)
	assert.Equal(t, masterID.String(), aReq.CommitID)
	assert.Equal(t, storageCommitPrefix+aReq.CommitID, aReq.StorageCommitID())

	aReq, err = NewRequest(testRepoID, ctx.Repo.GitRepo, "test/archive.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, repo_model.ArchiveTarGz, aReq.Type)
	assert.Equal(t, "test-archive-sb.tar.gz", aReq.GetArchiveName())

	// Bundles are git-specific and never SB.
	_, err = NewRequest(testRepoID, ctx.Repo.GitRepo, "master.bundle")
	assert.ErrorIs(t, err, ErrUnknownArchiveFormat{})

	_, err = NewRequest(testRepoID, ctx.Repo.GitRepo, "master.unknown")
	assert.ErrorIs(t, err, ErrUnknownArchiveFormat{})

	_, err = NewRequest(testRepoID, ctx.Repo.GitRepo, "no-such-ref.zip")
	assert.ErrorIs(t, err, RepoRefNotFoundError{})
}

func TestGetRepoDMForArchive(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// No metadata rows at all and a repo name that yields no type: not archivable.
	_, err := getRepoDMForArchive(t.Context(), loadRepo(t), "master")
	assert.ErrorIs(t, err, ErrRepoNotConvertible{})

	// Default branch is RC; a second branch already holds converted SB content.
	insertDM(t, &repo_model.Door43Metadata{
		RepoID: testRepoID, Ref: "master", RefType: "branch", CommitSHA: testFirstCommit,
		Stage: door43metadata.StageLatest, IsLatestForStage: true, MetadataType: "rc",
	})
	insertDM(t, &repo_model.Door43Metadata{
		RepoID: testRepoID, Ref: "test/archive", RefType: "branch", CommitSHA: "aacbdfe9e1c4",
		Stage: door43metadata.StageOther, MetadataType: "sb",
	})

	dm, err := getRepoDMForArchive(t.Context(), loadRepo(t), "master")
	require.NoError(t, err)
	assert.Equal(t, "rc", dm.MetadataType)

	// The requested ref's own row decides, not the default branch.
	dm, err = getRepoDMForArchive(t.Context(), loadRepo(t), "test/archive")
	require.NoError(t, err)
	assert.Equal(t, "sb", dm.MetadataType)

	// A bare commit ID has no row of its own and falls back to the default branch row.
	dm, err = getRepoDMForArchive(t.Context(), loadRepo(t), testFirstCommit)
	require.NoError(t, err)
	assert.Equal(t, "rc", dm.MetadataType)

	// A ref row with an unsupported type also falls back rather than failing.
	insertDM(t, &repo_model.Door43Metadata{
		RepoID: testRepoID, Ref: "v0", RefType: "tag", CommitSHA: testFirstCommit,
		Stage: door43metadata.StageProd, MetadataType: "",
	})
	dm, err = getRepoDMForArchive(t.Context(), loadRepo(t), "v0")
	require.NoError(t, err)
	assert.Equal(t, "rc", dm.MetadataType)
}

func TestStreamSBPassThrough(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Repository.PrefixArchiveFiles, true)()

	insertDM(t, &repo_model.Door43Metadata{
		RepoID: testRepoID, Ref: "test/archive", RefType: "branch", CommitSHA: "aacbdfe9e1c4",
		Stage: door43metadata.StageOther, MetadataType: "sb",
	})

	ctx, _ := contexttest.MockContext(t, "user27/repo49")
	contexttest.LoadRepo(t, ctx, testRepoID)
	contexttest.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	t.Run("zip", func(t *testing.T) {
		aReq, err := NewRequest(testRepoID, ctx.Repo.GitRepo, "test/archive.zip")
		require.NoError(t, err)

		var buf bytes.Buffer
		require.NoError(t, aReq.Stream(t.Context(), ctx.Repo.Repository, &buf))

		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)
		var names []string
		for _, f := range zr.File {
			names = append(names, f.Name)
		}
		assertPassThroughNames(t, names)
	})

	t.Run("tar.gz", func(t *testing.T) {
		aReq, err := NewRequest(testRepoID, ctx.Repo.GitRepo, "test/archive.tar.gz")
		require.NoError(t, err)

		var buf bytes.Buffer
		require.NoError(t, aReq.Stream(t.Context(), ctx.Repo.Repository, &buf))

		gzr, err := gzip.NewReader(&buf)
		require.NoError(t, err)
		tr := tar.NewReader(gzr)
		var names []string
		for {
			hdr, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
			if hdr.Typeflag == tar.TypeXGlobalHeader { // git archive records the commit ID here
				continue
			}
			names = append(names, hdr.Name)
		}
		assertPassThroughNames(t, names)
	})
}

// assertPassThroughNames checks the archive is the ref's tree under the repo-name prefix
// with no conversion output and no git internals.
func assertPassThroughNames(t *testing.T, names []string) {
	assert.Contains(t, names, "repo49/README.md")
	assert.Contains(t, names, "repo49/test/test.txt")
	for _, name := range names {
		assert.True(t, strings.HasPrefix(name, "repo49/"), "unexpected entry %q", name)
		assert.False(t, strings.HasPrefix(name, "repo49/.git/"), "git internals leaked: %q", name)
		assert.NotEqual(t, "repo49/metadata.json", name, "conversion ran on an SB repo")
	}
}
