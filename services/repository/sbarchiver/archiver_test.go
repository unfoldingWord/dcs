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
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
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

	aReq, err := NewRequest(ctx, testRepoID, ctx.Repo.GitRepo, "master.zip")
	require.NoError(t, err)
	assert.Equal(t, repo_model.ArchiveZip, aReq.Type)
	assert.Equal(t, "master-sb.zip", aReq.GetArchiveName())
	masterID, err := ctx.Repo.GitRepo.ConvertToGitID(ctx, "master")
	require.NoError(t, err)
	assert.Equal(t, masterID.String(), aReq.CommitID)
	assert.Equal(t, storageCommitPrefix+aReq.CommitID, aReq.StorageCommitID())

	aReq, err = NewRequest(ctx, testRepoID, ctx.Repo.GitRepo, "test/archive.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, repo_model.ArchiveTarGz, aReq.Type)
	assert.Equal(t, "test-archive-sb.tar.gz", aReq.GetArchiveName())

	// Bundles are git-specific and never SB.
	_, err = NewRequest(ctx, testRepoID, ctx.Repo.GitRepo, "master.bundle")
	assert.ErrorIs(t, err, ErrUnknownArchiveFormat{})

	_, err = NewRequest(ctx, testRepoID, ctx.Repo.GitRepo, "master.unknown")
	assert.ErrorIs(t, err, ErrUnknownArchiveFormat{})

	_, err = NewRequest(ctx, testRepoID, ctx.Repo.GitRepo, "no-such-ref.zip")
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
		aReq, err := NewRequest(ctx, testRepoID, ctx.Repo.GitRepo, "test/archive.zip")
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
		aReq, err := NewRequest(ctx, testRepoID, ctx.Repo.GitRepo, "test/archive.tar.gz")
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

// TestPayloadRefForArchive uses user27/template1 (repo 44, git branches: "master" only) as
// the payload repo of repo49 and checks which of its refs each requested ref pairs with.
func TestPayloadRefForArchive(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	const payloadRepoID int64 = 44
	payloadRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: payloadRepoID})
	payloadGitRepo, err := git.OpenRepository(t.Context(), payloadRepo)
	require.NoError(t, err)
	defer payloadGitRepo.Close()

	// Payload repo releases around the requested repo's v1 (released at 1000) and v2 (at 5000).
	for _, dm := range []*repo_model.Door43Metadata{
		{Ref: "v1", RefType: "tag", Stage: door43metadata.StageProd, ReleaseDateUnix: 900},
		{Ref: "v1.1", RefType: "tag", Stage: door43metadata.StagePreProd, ReleaseDateUnix: 1010},
		{Ref: "v2", RefType: "tag", Stage: door43metadata.StageProd, ReleaseDateUnix: 4000, IsLatestForStage: true},
		{Ref: "master", RefType: "branch", Stage: door43metadata.StageLatest, ReleaseDateUnix: 6000, IsLatestForStage: true},
	} {
		dm.RepoID, dm.CommitSHA, dm.MetadataType, dm.Language = payloadRepoID, "2a83b34", "rc", "en"
		insertDM(t, dm)
	}

	requested := func(ref, refType string, stage door43metadata.Stage, released timeutil.TimeStamp) *repo_model.Door43Metadata {
		return &repo_model.Door43Metadata{
			RepoID: testRepoID, Repo: loadRepo(t), Ref: ref, RefType: refType, CommitSHA: testFirstCommit,
			Stage: stage, MetadataType: "rc", Language: "en", ReleaseDateUnix: released,
		}
	}

	// A production release pairs with the payload's production release closest in time,
	// not its latest release and not the pre-release that is even closer.
	ref, err := payloadRefForArchive(t.Context(), requested("v1", "tag", door43metadata.StageProd, 1000), "v1", payloadRepo, payloadGitRepo)
	require.NoError(t, err)
	assert.Equal(t, "v1", ref)

	ref, err = payloadRefForArchive(t.Context(), requested("v2", "tag", door43metadata.StageProd, 5000), "v2", payloadRepo, payloadGitRepo)
	require.NoError(t, err)
	assert.Equal(t, "v2", ref)

	// A pre-release may pair with a pre-release.
	ref, err = payloadRefForArchive(t.Context(), requested("v1.1-rc", "tag", door43metadata.StagePreProd, 1020), "v1.1-rc", payloadRepo, payloadGitRepo)
	require.NoError(t, err)
	assert.Equal(t, "v1.1", ref)

	// Another language has no book package entry and the payload has no such tag: default branch.
	fr := requested("v3", "tag", door43metadata.StageProd, 1000)
	fr.Language = "fr"
	ref, err = payloadRefForArchive(t.Context(), fr, "v3", payloadRepo, payloadGitRepo)
	require.NoError(t, err)
	assert.Equal(t, "master", ref)

	// A branch pairs with the payload's same-named branch when it has one, else its default branch.
	ref, err = payloadRefForArchive(t.Context(), requested("test/archive", "branch", door43metadata.StageOther, 0), "test/archive", payloadRepo, payloadGitRepo)
	require.NoError(t, err)
	assert.Equal(t, "master", ref)

	selfGitRepo, err := git.OpenRepository(t.Context(), loadRepo(t))
	require.NoError(t, err)
	defer selfGitRepo.Close()
	ref, err = payloadRefForArchive(t.Context(), requested("test/archive", "branch", door43metadata.StageOther, 0), "test/archive", loadRepo(t), selfGitRepo)
	require.NoError(t, err)
	assert.Equal(t, "test/archive", ref)

	// A bare commit falls back to the default branch metadata, whose ref is not the request: default branch.
	ref, err = payloadRefForArchive(t.Context(), requested("master", "branch", door43metadata.StageLatest, 0), testFirstCommit, payloadRepo, payloadGitRepo)
	require.NoError(t, err)
	assert.Equal(t, "master", ref)
}
