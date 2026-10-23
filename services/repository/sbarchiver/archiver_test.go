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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
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

	_, err := NewRequest(ctx, loadRepo(t), ctx.Repo.GitRepo, "master.zip")
	assert.ErrorIs(t, err, ErrRepoNotConvertible{})
	insertDM(t, &repo_model.Door43Metadata{
		RepoID: testRepoID, Ref: "master", RefType: "branch", CommitSHA: testFirstCommit,
		Stage: door43metadata.StageLatest, IsLatestForStage: true, MetadataType: "rc",
		Language: "en", Subject: "TSV Translation Notes",
	})

	aReq, err := NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "master.zip")
	require.NoError(t, err)
	assert.Equal(t, repo_model.ArchiveZip, aReq.Type)
	assert.Equal(t, "master-sb.zip", aReq.GetArchiveName())
	masterID, err := ctx.Repo.GitRepo.ConvertToGitID(ctx, "master")
	require.NoError(t, err)
	assert.Equal(t, masterID.String(), aReq.CommitID)
	assert.Equal(t, "rc", aReq.MetadataType)
	assert.True(t, strings.HasPrefix(aReq.StorageCommitID(), storageCommitPrefix))

	aReq, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "test/archive.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, repo_model.ArchiveTarGz, aReq.Type)
	assert.Equal(t, "test-archive-sb.tar.gz", aReq.GetArchiveName())

	// Bundles are git-specific and never SB.
	_, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "master.bundle")
	assert.ErrorIs(t, err, ErrUnknownArchiveFormat{})

	_, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "master.unknown")
	assert.ErrorIs(t, err, ErrUnknownArchiveFormat{})

	_, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "no-such-ref.zip")
	assert.ErrorIs(t, err, RepoRefNotFoundError{})
	// A full commit ID is passed through by ConvertToGitID, so it must be looked up too
	_, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, strings.Repeat("0", len(masterID.String()))+".zip")
	assert.ErrorIs(t, err, RepoRefNotFoundError{})

	// The response headers need to know whether the ref can move, and the commit time
	aReq, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "master.zip")
	require.NoError(t, err)
	assert.False(t, aReq.refIsCommitID)
	assert.False(t, aReq.commitTime.IsZero())
	aReq, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, masterID.String()+".zip")
	require.NoError(t, err)
	assert.True(t, aReq.refIsCommitID)

	// An annotated tag shares its commit's archive
	_, _, err = gitcmd.NewCommand("tag", "-a", "-m", "release", "v-annotated", "master").WithRepo(ctx.Repo.GitRepo).RunStdString(t.Context())
	require.NoError(t, err)
	aReq, err = NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "v-annotated.zip")
	require.NoError(t, err)
	assert.Equal(t, masterID.String(), aReq.CommitID)
	assert.False(t, aReq.refIsCommitID)
}

func TestCacheControl(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := loadRepo(t)
	const immutable = "public, max-age=31536000, immutable"
	const revalidate = "max-age=0, public, must-revalidate"

	// Only a commit ID with its own metadata and without partner content can't change: a tag can be made
	// again, metadata borrowed from the default branch can change, a partner's ref can move, and a partner
	// repo missing today can be created
	for _, c := range []struct {
		refIsCommitID   bool
		ownMetadata     bool
		mayHavePartner  bool
		payloadCommitID string
		want            string
	}{
		{true, true, false, "", immutable},
		{false, true, false, "", revalidate},
		{true, false, false, "", revalidate},
		{true, true, true, "", revalidate},
		{true, true, true, "0123456789abcdef0123456789abcdef01234567", revalidate},
		{false, true, true, "0123456789abcdef0123456789abcdef01234567", revalidate},
	} {
		aReq := &ArchiveRequest{
			refIsCommitID: c.refIsCommitID, ownMetadata: c.ownMetadata, mayHavePartner: c.mayHavePartner,
			PayloadCommitID: c.payloadCommitID, commitTime: time.Unix(1000, 0),
		}
		cacheControl, err := aReq.cacheControl(t.Context(), repo)
		require.NoError(t, err)
		assert.Equal(t, c.want, cacheControl, "%+v", c)
		// Last-Modified follows the same rule: a moved ref or a new partner commit doesn't change the commit time
		assert.Equal(t, c.want == immutable, !aReq.lastModified().IsZero(), "Last-Modified: %+v", c)
	}

	repo.IsPrivate = true
	cacheControl, err := (&ArchiveRequest{refIsCommitID: true, ownMetadata: true}).cacheControl(t.Context(), repo)
	require.NoError(t, err)
	assert.Equal(t, "private, max-age=31536000, immutable", cacheControl)
}

func TestStorageCommitID(t *testing.T) {
	// Every input that decides the bytes gets its own key, within repo_archiver's VARCHAR(64) for SHA-256 too
	keys := map[string]bool{}
	for _, commitID := range []string{strings.Repeat("a", 40), strings.Repeat("a", 64)} {
		for _, metadataType := range []string{"rc", "sb"} {
			for _, payloadCommitID := range []string{"", strings.Repeat("b", 40), strings.Repeat("b", 64)} {
				key := (&ArchiveRequest{CommitID: commitID, MetadataType: metadataType, PayloadCommitID: payloadCommitID}).StorageCommitID()
				assert.True(t, strings.HasPrefix(key, storageCommitPrefix), key)
				assert.LessOrEqual(t, len(key), 64, key)
				keys[key] = true
			}
		}
	}
	assert.Len(t, keys, 12)
}

// TestWriteDirectoryArchiveDeterministic checks that converted archives carry the commit time, not the
// conversion's file times, so that a commit always archives to the same bytes.
func TestWriteDirectoryArchiveDeterministic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ingredients"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "metadata.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ingredients", "TIT.tsv"), []byte("Reference\tNote\n"), 0o644))
	commitTime := time.Date(2026, 10, 22, 15, 6, 43, 0, time.FixedZone("CDT", -5*60*60))

	for _, archiveType := range []repo_model.ArchiveType{repo_model.ArchiveZip, repo_model.ArchiveTarGz} {
		var first, second bytes.Buffer
		require.NoError(t, writeDirectoryArchive(t.Context(), dir, "en_tn", archiveType, commitTime, &first))
		later := time.Now().Add(time.Hour)
		require.NoError(t, os.Chtimes(filepath.Join(dir, "metadata.json"), later, later))
		require.NoError(t, os.Chtimes(filepath.Join(dir, "ingredients"), later, later))
		require.NoError(t, writeDirectoryArchive(t.Context(), dir, "en_tn", archiveType, commitTime, &second))
		assert.Equal(t, first.Bytes(), second.Bytes(), archiveType.String())

		if archiveType == repo_model.ArchiveZip {
			zr, err := zip.NewReader(bytes.NewReader(first.Bytes()), int64(first.Len()))
			require.NoError(t, err)
			assert.Len(t, zr.File, 4) // en_tn/, en_tn/ingredients/ and the two files
			for _, f := range zr.File {
				assert.True(t, commitTime.Equal(f.Modified), "%s: %v", f.Name, f.Modified)
			}
		}
	}
}

func TestGetRepoDMForArchive(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// No metadata rows at all and a repo name that yields no type: not archivable.
	_, err := getRepoDMForArchive(t.Context(), loadRepo(t), "master", "")
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

	dm, err := getRepoDMForArchive(t.Context(), loadRepo(t), "master", "")
	require.NoError(t, err)
	assert.Equal(t, "rc", dm.MetadataType)

	// The requested ref's own row decides, not the default branch.
	dm, err = getRepoDMForArchive(t.Context(), loadRepo(t), "test/archive", "")
	require.NoError(t, err)
	assert.Equal(t, "sb", dm.MetadataType)

	// A bare commit ID has no row of its own and falls back to the default branch row.
	dm, err = getRepoDMForArchive(t.Context(), loadRepo(t), testFirstCommit, "")
	require.NoError(t, err)
	assert.Equal(t, "rc", dm.MetadataType)

	// A ref row with an unsupported type also falls back rather than failing.
	insertDM(t, &repo_model.Door43Metadata{
		RepoID: testRepoID, Ref: "v0", RefType: "tag", CommitSHA: testFirstCommit,
		Stage: door43metadata.StageProd, MetadataType: "",
	})
	dm, err = getRepoDMForArchive(t.Context(), loadRepo(t), "v0", "")
	require.NoError(t, err)
	assert.Equal(t, "rc", dm.MetadataType)

	// A commit ID reads like the production release whose head it is, before a newer pre-release or a
	// branch at the same commit
	const headCommit = "0123456789abcdef0123456789abcdef01234567"
	for _, row := range []*repo_model.Door43Metadata{
		{Ref: "v1", RefType: "tag", Stage: door43metadata.StageProd, MetadataType: "sb", ReleaseDateUnix: 1000},
		{Ref: "v2-rc", RefType: "tag", Stage: door43metadata.StagePreProd, MetadataType: "rc", ReleaseDateUnix: 2000},
		{Ref: "v1-draft", RefType: "tag", Stage: door43metadata.StageOther, MetadataType: ""},
		{Ref: "release", RefType: "branch", Stage: door43metadata.StageOther, MetadataType: "rc"},
	} {
		row.RepoID, row.CommitSHA = testRepoID, headCommit
		insertDM(t, row)
	}
	dm, err = getRepoDMForArchive(t.Context(), loadRepo(t), headCommit, headCommit)
	require.NoError(t, err)
	assert.Equal(t, "v1", dm.Ref)
	dm, err = getRepoDMForArchive(t.Context(), loadRepo(t), headCommit[:10], headCommit)
	require.NoError(t, err)
	assert.Equal(t, "v1", dm.Ref)
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
		aReq, err := NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "test/archive.zip")
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
		aReq, err := NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, "test/archive.tar.gz")
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
	// A tag with no catalog entry, for the same-named fallbacks.
	require.NoError(t, payloadGitRepo.CreateTag(t.Context(), "v9", "master"))

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
	resolve := func(dm *repo_model.Door43Metadata, refShortName, refType string, payload *repo_model.Repository, payloadGit *git.Repository) git.RefName {
		ref, err := payloadRefForArchive(t.Context(), dm, refShortName, refType, payload, payloadGit)
		require.NoError(t, err)
		return ref
	}

	// A production release pairs with the payload's production release closest in time,
	// not its latest release and not the pre-release that is even closer.
	assert.Equal(t, git.RefNameFromTag("v1"), resolve(requested("v1", "tag", door43metadata.StageProd, 1000), "v1", "tag", payloadRepo, payloadGitRepo))
	assert.Equal(t, git.RefNameFromTag("v2"), resolve(requested("v2", "tag", door43metadata.StageProd, 5000), "v2", "tag", payloadRepo, payloadGitRepo))

	// A pre-release may pair with a pre-release.
	assert.Equal(t, git.RefNameFromTag("v1.1"), resolve(requested("v1.1-rc", "tag", door43metadata.StagePreProd, 1020), "v1.1-rc", "tag", payloadRepo, payloadGitRepo))

	// Another language has no book package entry: a same-named tag if the payload has one, else default branch.
	fr := requested("v3", "tag", door43metadata.StageProd, 1000)
	fr.Language = "fr"
	assert.Equal(t, git.RefNameFromBranch("master"), resolve(fr, "v3", "tag", payloadRepo, payloadGitRepo))
	fr.Ref = "v9"
	assert.Equal(t, git.RefNameFromTag("v9"), resolve(fr, "v9", "tag", payloadRepo, payloadGitRepo))

	// A branch never pairs with a same-named tag.
	assert.Equal(t, git.RefNameFromBranch("master"), resolve(requested("v9", "branch", door43metadata.StageOther, 0), "v9", "branch", payloadRepo, payloadGitRepo))

	// A branch pairs with the payload's same-named branch when it has one, else its default branch.
	assert.Equal(t, git.RefNameFromBranch("master"), resolve(requested("test/archive", "branch", door43metadata.StageOther, 0), "test/archive", "branch", payloadRepo, payloadGitRepo))

	selfRepo := loadRepo(t)
	selfGitRepo, err := git.OpenRepository(t.Context(), selfRepo)
	require.NoError(t, err)
	defer selfGitRepo.Close()
	assert.Equal(t, git.RefNameFromBranch("test/archive"), resolve(requested("test/archive", "branch", door43metadata.StageOther, 0), "test/archive", "branch", selfRepo, selfGitRepo))

	// A tag never pairs with a same-named branch.
	assert.Equal(t, git.RefNameFromBranch("master"), resolve(requested("test/archive", "tag", door43metadata.StageProd, 0), "test/archive", "tag", selfRepo, selfGitRepo))

	// A payload with a branch and a tag of the same name at different commits: each kind
	// of request resolves to its own kind, even though git's short-name lookup prefers the tag.
	_, _, err = gitcmd.NewCommand("branch", "both", "master").WithRepo(selfGitRepo).RunStdString(t.Context())
	require.NoError(t, err)
	require.NoError(t, selfGitRepo.CreateTag(t.Context(), "both", testFirstCommit))
	branchRef := resolve(requested("both", "branch", door43metadata.StageOther, 0), "both", "branch", selfRepo, selfGitRepo)
	tagRef := resolve(requested("both", "tag", door43metadata.StageProd, 0), "both", "tag", selfRepo, selfGitRepo)
	assert.Equal(t, git.RefNameFromBranch("both"), branchRef)
	assert.Equal(t, git.RefNameFromTag("both"), tagRef)
	branchID, err := selfGitRepo.ConvertToGitID(t.Context(), branchRef.String())
	require.NoError(t, err)
	tagID, err := selfGitRepo.ConvertToGitID(t.Context(), tagRef.String())
	require.NoError(t, err)
	masterID, err := selfGitRepo.ConvertToGitID(t.Context(), "master")
	require.NoError(t, err)
	assert.Equal(t, masterID.String(), branchID.String())
	assert.True(t, strings.HasPrefix(tagID.String(), testFirstCommit))
	shortID, err := selfGitRepo.ConvertToGitID(t.Context(), "both")
	require.NoError(t, err)
	assert.Equal(t, tagID.String(), shortID.String(), "git resolves the bare name to the tag; the archiver must not rely on it")

	// A bare commit falls back to the default branch metadata, whose ref is not the request: default branch.
	assert.Equal(t, git.RefNameFromBranch("master"), resolve(requested("master", "branch", door43metadata.StageLatest, 0), testFirstCommit, "", payloadRepo, payloadGitRepo))
}

func TestRequestedRefType(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := loadRepo(t)

	// The ref's own metadata row answers without touching git.
	refType, err := requestedRefType(t.Context(), repo, &repo_model.Door43Metadata{Ref: "v1", RefType: "tag"}, "v1")
	require.NoError(t, err)
	assert.Equal(t, "tag", refType)

	// Otherwise git decides: a branch, or a bare commit that is neither branch nor tag.
	fallbackDM := &repo_model.Door43Metadata{Ref: "master", RefType: "branch"}
	refType, err = requestedRefType(t.Context(), repo, fallbackDM, "test/archive")
	require.NoError(t, err)
	assert.Equal(t, "branch", refType)

	refType, err = requestedRefType(t.Context(), repo, fallbackDM, testFirstCommit)
	require.NoError(t, err)
	assert.Empty(t, refType)
}
