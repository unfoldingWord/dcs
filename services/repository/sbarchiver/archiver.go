// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sbarchiver

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"gitea.dev/models"
	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	"gitea.dev/modules/process"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	gitea_context "gitea.dev/services/context"

	rc2sb "github.com/unfoldingWord/go-rc2sb"
	tc2rc "github.com/unfoldingWord/go-tc2rc"
	ts2rc "github.com/unfoldingWord/go-ts2rc"
	"xorm.io/builder"
)

const storageCommitPrefix = "sb-"

// sbArchiveWriterVersion goes up whenever DCS changes what a conversion produces, in what writeZipArchive and
// writeTarGzArchive write or in what Stream passes the converters, so that archives kept from before are
// converted anew rather than served with an ETag that still matches. Repos already in SB format are archived
// by "git archive", so their keys leave it out.
const sbArchiveWriterVersion = 3

// converterVersions lists the versions of the modules that convert rc, ts and tc repos to SB, as built in
var converterVersions = sync.OnceValue(func() string {
	return converterVersionsFrom(debug.ReadBuildInfo())
})

func converterVersionsFrom(info *debug.BuildInfo, ok bool) string {
	var versions []string
	if ok {
		for _, dep := range info.Deps {
			switch dep.Path {
			case "github.com/unfoldingWord/go-rc2sb", "github.com/unfoldingWord/go-ts2rc", "github.com/unfoldingWord/go-tc2rc":
				versions = append(versions, dep.Path+"@"+dep.Version)
			}
		}
	}
	if len(versions) == 0 { // a binary without build info: its own version changes with every upgrade
		return "dcs@" + setting.AppVer
	}
	slices.Sort(versions)
	return strings.Join(versions, " ")
}

// ArchiveRequest defines the parameters of an SB archive request.
type ArchiveRequest struct {
	RepoID   int64
	Type     repo_model.ArchiveType
	CommitID string

	RefShortName string // "master", "v1.0.0", commit id, etc. Exported so it survives queue serialization.

	// MetadataRef is the ref whose metadata decides the conversion (see getRepoDMForArchive), and
	// MetadataType is its type then, which is part of the cache key: "sb" archives the tree as it is.
	MetadataRef  string
	MetadataType string
	// The partner repo commit whose content a TW or TWL burrito also carries (see payloadRefForArchive).
	// It is part of the cache key, since the partner ref paired with a branch can move.
	PayloadRef      string
	PayloadCommitID string
	// The canonical en_tw repo and commit a ts Translation Words conversion places its articles by (see
	// resolveTsTWSource). The commit is part of the cache key, since that repo's default branch can move.
	TWSourceRepoID   int64
	TWSourceCommitID string

	commitTime time.Time // set by NewRequest; a request from the queue looks it up
}

// ErrUnknownArchiveFormat means the requested archive format is not supported.
type ErrUnknownArchiveFormat struct {
	RequestNameType string
}

func (err ErrUnknownArchiveFormat) Error() string {
	return "unknown format: " + err.RequestNameType
}

func (ErrUnknownArchiveFormat) Is(err error) bool {
	_, ok := err.(ErrUnknownArchiveFormat)
	return ok
}

// RepoRefNotFoundError is returned when a requested reference was not found.
type RepoRefNotFoundError struct {
	RefShortName string
}

func (e RepoRefNotFoundError) Error() string {
	return "unrecognized repository reference: " + e.RefShortName
}

func (e RepoRefNotFoundError) Is(err error) bool {
	_, ok := err.(RepoRefNotFoundError)
	return ok
}

// ErrRepoNotConvertible means SB archive generation is only available for rc, ts, tc, and sb repositories.
type ErrRepoNotConvertible struct {
	RepoID int64
}

func (e ErrRepoNotConvertible) Error() string {
	return fmt.Sprintf("repository %d is not an rc, ts, tc, or sb repository", e.RepoID)
}

func (e ErrRepoNotConvertible) Is(err error) bool {
	_, ok := err.(ErrRepoNotConvertible)
	return ok
}

// NewRequest creates an SB archival request from URI suffix like "master.zip".
func NewRequest(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, archiveRefExt string) (*ArchiveRequest, error) {
	archiveRefShortName, archiveType := repo_model.SplitArchiveNameType(archiveRefExt)
	if archiveType == repo_model.ArchiveUnknown {
		return nil, ErrUnknownArchiveFormat{archiveRefExt}
	}
	if archiveType == repo_model.ArchiveBundle {
		return nil, ErrUnknownArchiveFormat{archiveRefExt}
	}
	return newRequest(ctx, repo, gitRepo, archiveRefShortName, archiveType)
}

func newRequest(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, refShortName string, archiveType repo_model.ArchiveType) (*ArchiveRequest, error) {
	// GetCommit peels an annotated tag to its commit, so a tag and its commit ID share one cached archive
	commit, err := gitRepo.GetCommit(ctx, refShortName)
	if err != nil {
		return nil, RepoRefNotFoundError{RefShortName: refShortName}
	}
	aReq := &ArchiveRequest{
		RepoID:       repo.ID,
		Type:         archiveType,
		CommitID:     commit.ID.String(),
		RefShortName: refShortName,
		commitTime:   commit.Committer.When,
	}

	headOf := ""
	if !gitRepo.IsBranchExist(ctx, refShortName) && !gitRepo.IsTagExist(ctx, refShortName) {
		headOf = aReq.CommitID
	}
	dm, err := getRepoDMForArchive(ctx, repo, refShortName, headOf)
	if err != nil {
		return nil, err
	}
	aReq.MetadataRef, aReq.MetadataType = dm.Ref, dm.MetadataType

	if dm.MetadataType == "ts" && dm.Subject == "Translation Words" {
		if aReq.TWSourceRepoID, aReq.TWSourceCommitID, err = resolveTsTWSource(ctx); err != nil {
			return nil, err
		}
	}

	if dm.MetadataType != "sb" {
		// A commit ID pairs with the partner as the release or branch it is the head of would
		payloadFor := refShortName
		if headOf != "" && dm.CommitSHA == headOf {
			payloadFor = dm.Ref
		}
		payloadRef, payloadCommitID, err := resolvePayload(ctx, repo, dm, payloadFor)
		if err != nil {
			return nil, err
		}
		aReq.PayloadRef, aReq.PayloadCommitID = payloadRef.String(), payloadCommitID
	}
	return aReq, nil
}

// GetArchiveName returns archive name based on requested ref.
func (aReq *ArchiveRequest) GetArchiveName() string {
	return strings.ReplaceAll(aReq.RefShortName, "/", "-") + "-sb." + aReq.Type.String()
}

// StorageCommitID returns the cache key used as the commit ID in repo_archiver. Everything that decides the
// archive's bytes is in it: the commit, the metadata type, the TW/TWL partner commit, the en_tw commit of a
// ts Translation Words conversion, whether the files sit in a prefix folder (PREFIX_ARCHIVE_FILES), and the
// converter and archive writer versions, so that an upgrade doesn't serve old conversions until the archive cleanup. It is
// hashed to fit the column's 64 characters, which "sb-" and a SHA-256 commit ID alone would exceed.
func (aReq *ArchiveRequest) StorageCommitID() string {
	key := aReq.CommitID + "\n" + aReq.MetadataType + "\n" + aReq.PayloadCommitID
	if aReq.TWSourceCommitID != "" {
		key += "\n" + aReq.TWSourceCommitID
	}
	if !setting.Repository.PrefixArchiveFiles {
		key += "\nunprefixed"
	}
	if aReq.MetadataType != "sb" {
		key += fmt.Sprintf("\n%s\nwriter %d", converterVersions(), sbArchiveWriterVersion)
	}
	sum := sha256.Sum256([]byte(key))
	return storageCommitPrefix + hex.EncodeToString(sum[:])[:60]
}

// Await waits for archive generation completion.
func (aReq *ArchiveRequest) Await(ctx context.Context) (*repo_model.RepoArchiver, error) {
	archiver, err := repo_model.GetRepoArchiver(ctx, aReq.RepoID, aReq.Type, aReq.StorageCommitID())
	if err != nil {
		return nil, fmt.Errorf("repo_model.GetRepoArchiver: %w", err)
	}

	if archiver != nil && archiver.Status == repo_model.ArchiverReady {
		return archiver, nil
	}

	// Try to satisfy the request immediately. If another process is already
	// generating it, doArchive returns nil and we fall back to polling.
	archiver, err = doArchive(ctx, aReq)
	if err != nil {
		return nil, fmt.Errorf("sbarchiver.doArchive: %w", err)
	}
	if archiver != nil && archiver.Status == repo_model.ArchiverReady {
		return archiver, nil
	}

	poll := time.NewTicker(time.Second)
	defer poll.Stop()

	for {
		select {
		case <-graceful.GetManager().HammerContext().Done():
			return nil, graceful.GetManager().HammerContext().Err()
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-poll.C:
			archiver, err = repo_model.GetRepoArchiver(ctx, aReq.RepoID, aReq.Type, aReq.StorageCommitID())
			if err != nil {
				return nil, fmt.Errorf("repo_model.GetRepoArchiver: %w", err)
			}
			if archiver != nil && archiver.Status == repo_model.ArchiverReady {
				return archiver, nil
			}
		}
	}
}

// Stream generates and streams an SB archive. Repos already in SB format are archived
// as-is via "git archive"; rc, ts, and tc repos are converted to SB first.
func (aReq *ArchiveRequest) Stream(ctx context.Context, repo *repo_model.Repository, w io.Writer) error {
	repoDM, err := getRepoDMForArchive(ctx, repo, util.IfZero(aReq.MetadataRef, aReq.RefShortName), "")
	if err != nil {
		return err
	}
	// Convert as the cache key says, even if the metadata has changed since the request was made
	repoDM.MetadataType = util.IfZero(aReq.MetadataType, repoDM.MetadataType)

	if repoDM.MetadataType == "sb" {
		return git.CreateArchive(ctx, repo, repo.Name, aReq.Type.String(), w, aReq.CommitID, nil)
	}

	tmpDir, cleanup, err := setting.AppDataTempDir("repo-sb-archive").MkdirTempRandom(fmt.Sprintf("%d-", repo.ID))
	if err != nil {
		return fmt.Errorf("create temporary directory: %w", err)
	}
	defer cleanup()

	// ts and tc repos are cloned to a separate dir and first converted to RC format below.
	rcDir := filepath.Join(tmpDir, "rc")
	cloneDir := rcDir
	isTS := repoDM.MetadataType == "ts"
	isTC := repoDM.MetadataType == "tc"
	if isTS {
		cloneDir = filepath.Join(tmpDir, "ts")
	} else if isTC {
		cloneDir = filepath.Join(tmpDir, "tc")
	}
	if err := cloneRepositoryAtCommit(ctx, gitrepo.RepoLocalPath(repo.CodeStorageRepo()), aReq.RefShortName, aReq.CommitID, cloneDir); err != nil {
		return fmt.Errorf("clone requested repo ref: %w", err)
	}

	// For ts repos, convert ts -> RC so the RC -> SB pipeline below applies unchanged.
	if isTS {
		twSourceDir, err := prepareTsTWSourceDir(ctx, tmpDir, repoDM, aReq)
		if err != nil {
			return err
		}
		rep := ts2rc.Convert(ctx, cloneDir, rcDir, ts2rc.Options{TWSourceDir: twSourceDir})
		if !rep.OK {
			return fmt.Errorf("ts2rc.Convert: %s", rep.Error)
		}
	}

	// For tc repos, convert tc -> RC so the RC -> SB pipeline below applies unchanged.
	// RepoName locates the exported USFM file (<repoName>.usfm) since the clone dir is "tc".
	if isTC {
		rep := tc2rc.Convert(ctx, cloneDir, rcDir, tc2rc.Options{RepoName: repo.Name})
		if !rep.OK {
			return fmt.Errorf("tc2rc.Convert: %s", rep.Error)
		}
	}

	payloadPath, err := preparePayloadPath(ctx, tmpDir, rcDir, repo, repoDM, aReq)
	if err != nil {
		return err
	}

	// The commit time stamps metadata.json and every archive entry, so a commit always converts to the same bytes
	modTime, err := aReq.getCommitTime(ctx, repo)
	if err != nil {
		return err
	}
	sbDir := filepath.Join(tmpDir, "sb")
	upstream, err := upstreamRevisions(ctx, repo, repoDM, aReq)
	if err != nil {
		return err
	}
	// The clone of a ts or tc repo was converted to rc first, so it is no git clone that rc2sb could read these from
	_, err = rc2sb.Convert(ctx, rcDir, sbDir, rc2sb.Options{
		PayloadPath: payloadPath,
		Owner:       repo.OwnerName,
		RepoName:    repo.Name,
		DCSURL:      strings.TrimSuffix(setting.AppURL, "/"),
		Revision:    aReq.CommitID,
		Timestamp:   modTime,
		Upstream:    upstream,
	})
	if err != nil {
		return fmt.Errorf("rc2sb.Convert: %w", err)
	}

	prefixDir := ""
	if setting.Repository.PrefixArchiveFiles {
		prefixDir = repo.Name
	}
	if err := writeDirectoryArchive(ctx, sbDir, prefixDir, aReq.Type, modTime, w); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}

	return nil
}

// getCommitTime returns the committer time of the archived commit. It stamps every entry of a
// converted archive, as "git archive" does, so that the entries don't carry the conversion time.
func (aReq *ArchiveRequest) getCommitTime(ctx context.Context, repo *repo_model.Repository) (time.Time, error) {
	if !aReq.commitTime.IsZero() {
		return aReq.commitTime, nil
	}
	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return time.Time{}, fmt.Errorf("git.OpenRepository(%s): %w", repo.FullName(), err)
	}
	defer gitRepo.Close()
	commit, err := gitRepo.GetCommit(ctx, aReq.CommitID)
	if err != nil {
		return time.Time{}, fmt.Errorf("gitRepo.GetCommit(%s): %w", aReq.CommitID, err)
	}
	aReq.commitTime = commit.Committer.When
	return aReq.commitTime, nil
}

var archivableMetadataTypes = []string{"rc", "ts", "tc", "sb"}

// isArchivableMetadataType reports whether SB archives can be produced for the metadata type.
func isArchivableMetadataType(metadataType string) bool {
	return slices.Contains(archivableMetadataTypes, metadataType)
}

// getRepoDMForArchive returns the Door43Metadata that decides how the requested ref is
// archived. The ref's own row wins (a repo's "main" branch may be SB while its default
// branch is RC). A commit ID, passed as headOf, takes the row of a tag or branch whose head
// it is, a production release's first, so that it reads like the release it is. An unprocessed ref falls
// back to the default branch metadata, then the repo-level metadata.
func getRepoDMForArchive(ctx context.Context, repo *repo_model.Repository, refShortName, headOf string) (*repo_model.Door43Metadata, error) {
	if refShortName != "" {
		refDM, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, refShortName)
		if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
			return nil, fmt.Errorf("repo_model.GetDoor43MetadataByRepoIDAndRef: %w", err)
		}
		if refDM != nil && isArchivableMetadataType(refDM.MetadataType) {
			refDM.Repo = repo
			return refDM, nil
		}
	}

	if headOf != "" {
		headDM := &repo_model.Door43Metadata{}
		has, err := db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repo.ID, "commit_sha": headOf}).
			And(builder.In("metadata_type", archivableMetadataTypes)).
			// A tag before a branch ("tag" sorts after "branch"), then prod before preprod and the rest
			OrderBy("ref_type DESC, CASE WHEN stage = 0 THEN 9 ELSE stage END, release_date_unix DESC, id DESC").
			Get(headDM)
		if err != nil {
			return nil, fmt.Errorf("find the metadata of commit %s: %w", headOf, err)
		}
		if has {
			headDM.Repo = repo
			return headDM, nil
		}
	}

	if err := repo.LoadLatestDMs(ctx); err != nil {
		return nil, fmt.Errorf("repo.LoadLatestDMs: %w", err)
	}

	dm := repo.DefaultBranchDM
	if dm == nil {
		dm = repo.RepoDM
	}
	if dm == nil || !isArchivableMetadataType(dm.MetadataType) {
		return nil, ErrRepoNotConvertible{RepoID: repo.ID}
	}
	return dm, nil
}

// tsTWSourceOwners are tried in order when locating a canonical en_tw repo. ts2rc uses it
// to place Translation Words articles under bible/{kt|names|other}/ by matching slug filenames.
var tsTWSourceOwners = []string{"unfoldingWord", "Door43-Catalog"}

// resolveTsTWSource finds the canonical en_tw repo a ts Translation Words conversion reads and its
// default branch's commit now, or returns (0, "", nil) when no canonical en_tw repo exists on this server.
func resolveTsTWSource(ctx context.Context) (int64, string, error) {
	for _, owner := range tsTWSourceOwners {
		twRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, owner, "en_tw")
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				continue
			}
			return 0, "", fmt.Errorf("repo_model.GetRepositoryByOwnerAndName(%s/en_tw): %w", owner, err)
		}

		twGitRepo, err := git.OpenRepository(ctx, twRepo)
		if err != nil {
			return 0, "", fmt.Errorf("git.OpenRepository(%s): %w", twRepo.FullName(), err)
		}
		commitID, err := twGitRepo.ConvertToGitID(ctx, twRepo.DefaultBranch)
		twGitRepo.Close()
		if err != nil {
			return 0, "", fmt.Errorf("resolve %s default branch: %w", twRepo.FullName(), err)
		}
		return twRepo.ID, commitID.String(), nil
	}
	return 0, "", nil
}

// prepareTsTWSourceDir clones the en_tw commit the request resolved (see resolveTsTWSource) into
// tmpDir/tw-source for ts Translation Words conversions and returns its path. Returns ("", nil) when
// the repo is not a ts TW repo or no canonical en_tw repo was found — ts2rc then falls back to
// writing articles under bible/other/ with a warning.
func prepareTsTWSourceDir(ctx context.Context, tmpDir string, dm *repo_model.Door43Metadata, aReq *ArchiveRequest) (string, error) {
	if dm == nil || dm.Subject != "Translation Words" {
		return "", nil
	}
	if aReq.TWSourceRepoID == 0 {
		log.Warn("SB archive: no canonical en_tw repo found for TW category lookup; articles will fall back to bible/other/")
		return "", nil
	}

	twRepo, err := repo_model.GetRepositoryByID(ctx, aReq.TWSourceRepoID)
	if err != nil {
		return "", fmt.Errorf("repo_model.GetRepositoryByID(%d): %w", aReq.TWSourceRepoID, err)
	}
	twSourceDir := filepath.Join(tmpDir, "tw-source")
	if err := cloneRepositoryAtCommit(ctx, gitrepo.RepoLocalPath(twRepo.CodeStorageRepo()), twRepo.DefaultBranch, aReq.TWSourceCommitID, twSourceDir); err != nil {
		return "", fmt.Errorf("clone TW source repo %s: %w", twRepo.FullName(), err)
	}
	return twSourceDir, nil
}

// requestedRefType reports whether refShortName names a branch or a tag of repo
// ("branch", "tag"), or "" for anything else such as a bare commit ID. The ref's own
// metadata row answers when it has one; otherwise the git repository is consulted.
func requestedRefType(ctx context.Context, repo *repo_model.Repository, dm *repo_model.Door43Metadata, refShortName string) (string, error) {
	if dm.Ref == refShortName && (dm.RefType == "branch" || dm.RefType == "tag") {
		return dm.RefType, nil
	}
	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return "", fmt.Errorf("git.OpenRepository(%s): %w", repo.FullName(), err)
	}
	defer gitRepo.Close()
	switch {
	case gitRepo.IsBranchExist(ctx, refShortName):
		return "branch", nil
	case gitRepo.IsTagExist(ctx, refShortName):
		return "tag", nil
	default:
		return "", nil
	}
}

// payloadRefForArchive picks the ref of payloadRepo to pair with the requested ref of the
// repo being archived, so a release's archive carries the payload as it was when that
// release was made rather than whatever the payload repo's default branch holds today:
//   - a release tag: the payload repo's entry in the requested ref's book package (its
//     release closest in time to the requested one), else a same-named tag
//   - a branch: the same-named branch when the payload repo has one
//   - otherwise (a bare commit, no match above): the payload repo's default branch
//
// The result is fully qualified (refs/tags/..., refs/heads/...) so that resolving it is
// unambiguous when the payload repo has a branch and a tag of the same name.
//
// refType is the requested ref's kind from requestedRefType. dm is the requested repo's
// metadata as chosen by getRepoDMForArchive; it describes refShortName itself unless
// that ref has no metadata of its own, in which case the book package is not consulted.
func payloadRefForArchive(ctx context.Context, dm *repo_model.Door43Metadata, refShortName, refType string, payloadRepo *repo_model.Repository, payloadGitRepo *git.Repository) (git.RefName, error) {
	switch refType {
	case "tag":
		if dm.Ref == refShortName {
			ref, err := bookPackageRef(ctx, dm, payloadRepo)
			if err != nil {
				return "", err
			}
			if ref != "" {
				return ref, nil
			}
		}
		if payloadGitRepo.IsTagExist(ctx, refShortName) {
			return git.RefNameFromTag(refShortName), nil
		}
	case "branch":
		if payloadGitRepo.IsBranchExist(ctx, refShortName) {
			return git.RefNameFromBranch(refShortName), nil
		}
	}
	return git.RefNameFromBranch(payloadRepo.DefaultBranch), nil
}

// bookPackageRef returns the ref of payloadRepo that the book package of dm's release
// pairs with it (what /api/v1/catalog/bp/{owner}/{repo}/{ref} reports), or "" when the
// book package has no entry for payloadRepo.
func bookPackageRef(ctx context.Context, dm *repo_model.Door43Metadata, payloadRepo *repo_model.Repository) (git.RefName, error) {
	if err := dm.LoadRepo(ctx); err != nil {
		return "", fmt.Errorf("dm.LoadRepo: %w", err)
	}
	opts := &door43metadata.SearchCatalogOptions{
		Repos:          []string{payloadRepo.Name},
		IncludeHistory: true,
	}
	dms, _, err := models.SearchCatalogForBookPackage(ctx, dm, opts)
	if err != nil {
		return "", fmt.Errorf("models.SearchCatalogForBookPackage(%s/%s): %w", dm.Repo.FullName(), dm.Ref, err)
	}
	for _, bpDM := range dms {
		if bpDM.RepoID != payloadRepo.ID {
			continue
		}
		if bpDM.RefType == "tag" {
			return git.RefNameFromTag(bpDM.Ref), nil
		}
		return git.RefNameFromBranch(bpDM.Ref), nil
	}
	return "", nil
}

var languageCodeRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// payloadRepoNameFor names the partner repo whose content a converted burrito of dm also carries:
// a TWL burrito holds the TW articles, and a TW burrito (isTWRepo) holds the TWL TSVs.
func payloadRepoNameFor(dm *repo_model.Door43Metadata) (payloadRepoName string, isTWRepo bool) {
	// The name becomes a folder of the conversion, so it must be a plain language code
	if !languageCodeRe.MatchString(dm.Language) {
		return "", false
	}
	switch dm.Subject {
	case "TSV Translation Words Links":
		return dm.Language + "_tw", false
	case "Translation Words":
		return dm.Language + "_twl", true
	}
	return "", false
}

// resolvePayload picks the commit of the partner repo whose content dm's burrito also carries, or
// returns "" when it has none or the partner repo doesn't exist.
func resolvePayload(ctx context.Context, repo *repo_model.Repository, dm *repo_model.Door43Metadata, refShortName string) (git.RefName, string, error) {
	payloadRepoName, _ := payloadRepoNameFor(dm)
	if payloadRepoName == "" {
		return "", "", nil
	}
	payloadRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, repo.OwnerName, payloadRepoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("repo_model.GetRepositoryByOwnerAndName(%s): %w", payloadRepoName, err)
	}

	payloadGitRepo, err := git.OpenRepository(ctx, payloadRepo)
	if err != nil {
		return "", "", fmt.Errorf("git.OpenRepository(%s): %w", payloadRepo.FullName(), err)
	}
	defer payloadGitRepo.Close()

	refType, err := requestedRefType(ctx, repo, dm, refShortName)
	if err != nil {
		return "", "", err
	}
	payloadRef, err := payloadRefForArchive(ctx, dm, refShortName, refType, payloadRepo, payloadGitRepo)
	if err != nil {
		return "", "", err
	}
	// GetCommit peels an annotated tag, so metadata.json names the partner's commit, not its tag object
	commit, err := payloadGitRepo.GetCommit(ctx, payloadRef.String())
	if err != nil {
		return "", "", fmt.Errorf("resolve %s ref %s: %w", payloadRepo.FullName(), payloadRef, err)
	}
	log.Debug("SB archive of %s@%s: using payload %s@%s (%s)", repo.FullName(), refShortName, payloadRepo.FullName(), payloadRef, commit.ID.String())
	return payloadRef, commit.ID.String(), nil
}

// upstreamRevisions lists the other repo commits a conversion of aReq takes content from, for metadata.json's
// identification.upstream: the TW/TWL partner commit, and the en_tw commit a ts Translation Words
// conversion places its articles by.
func upstreamRevisions(ctx context.Context, repo *repo_model.Repository, dm *repo_model.Door43Metadata, aReq *ArchiveRequest) ([]rc2sb.Upstream, error) {
	var upstream []rc2sb.Upstream
	if aReq.PayloadCommitID != "" {
		payloadRepoName, _ := payloadRepoNameFor(dm)
		payloadRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, repo.OwnerName, payloadRepoName)
		if err != nil {
			return nil, fmt.Errorf("upstreamRevisions: %w", err)
		}
		u, err := upstreamRevision(ctx, payloadRepo, aReq.PayloadCommitID)
		if err != nil {
			return nil, err
		}
		upstream = append(upstream, u)
	}
	if aReq.TWSourceCommitID != "" {
		twRepo, err := repo_model.GetRepositoryByID(ctx, aReq.TWSourceRepoID)
		if err != nil {
			return nil, fmt.Errorf("upstreamRevisions: %w", err)
		}
		u, err := upstreamRevision(ctx, twRepo, aReq.TWSourceCommitID)
		if err != nil {
			return nil, err
		}
		upstream = append(upstream, u)
	}
	return upstream, nil
}

func upstreamRevision(ctx context.Context, repo *repo_model.Repository, commitID string) (rc2sb.Upstream, error) {
	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return rc2sb.Upstream{}, fmt.Errorf("upstreamRevision(%s): %w", repo.FullName(), err)
	}
	defer gitRepo.Close()
	commit, err := gitRepo.GetCommit(ctx, commitID)
	if err != nil {
		return rc2sb.Upstream{}, fmt.Errorf("upstreamRevision(%s@%s): %w", repo.FullName(), commitID, err)
	}
	return rc2sb.Upstream{Owner: repo.OwnerName, RepoName: repo.Name, Revision: commitID, Timestamp: commit.Committer.When}, nil
}

// preparePayloadPath clones the partner commit of aReq for rc2sb: into rcDir/<lang>_twl/ for a TW repo,
// which rc2sb finds there, otherwise into its own folder, whose path is returned.
func preparePayloadPath(ctx context.Context, tmpDir, rcDir string, repo *repo_model.Repository, dm *repo_model.Door43Metadata, aReq *ArchiveRequest) (string, error) {
	payloadRepoName, isTWRepo := payloadRepoNameFor(dm)
	if payloadRepoName == "" || aReq.PayloadCommitID == "" {
		return "", nil
	}
	payloadRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, repo.OwnerName, payloadRepoName)
	if err != nil {
		return "", fmt.Errorf("repo_model.GetRepositoryByOwnerAndName(%s): %w", payloadRepoName, err)
	}

	destDir := filepath.Join(tmpDir, "payload")
	if isTWRepo {
		destDir = filepath.Join(rcDir, payloadRepoName)
	}
	if err := cloneRepositoryAtCommit(ctx, gitrepo.RepoLocalPath(payloadRepo.CodeStorageRepo()), git.RefName(aReq.PayloadRef).ShortName(), aReq.PayloadCommitID, destDir); err != nil {
		return "", fmt.Errorf("clone payload repository %s: %w", payloadRepo.FullName(), err)
	}
	if isTWRepo {
		return "", nil
	}
	return destDir, nil
}

// cloneRepositoryAtCommit clones sourceRepoPath into destination and checks out commitID.
// It first tries a shallow clone of refShortName; "git clone -b" takes a short name and
// prefers a branch over a same-named tag, so when the shallow clone does not contain
// commitID (or fails) it falls back to a full clone.
func cloneRepositoryAtCommit(ctx context.Context, sourceRepoPath, refShortName, commitID, destination string) error {
	tryShallow := refShortName != "" && commitID != ""
	if tryShallow {
		err := git.Clone(ctx, sourceRepoPath, destination, git.CloneRepoOptions{
			Quiet:  true,
			Depth:  1,
			Branch: refShortName,
		})
		if err == nil {
			err = checkoutCommit(ctx, destination, commitID)
			if err == nil {
				return nil
			}
			log.Debug("cloneRepositoryAtCommit: shallow clone of %q lacks %s, falling back to a full clone: %v", refShortName, commitID, err)
		}
		if removeErr := util.RemoveAllWithRetry(destination); removeErr != nil {
			log.Warn("cloneRepositoryAtCommit: failed to clean shallow clone destination %q: %v", destination, removeErr)
		}
	}

	if err := git.Clone(ctx, sourceRepoPath, destination, git.CloneRepoOptions{
		Quiet: true,
	}); err != nil {
		return err
	}

	if commitID == "" {
		return nil
	}
	return checkoutCommit(ctx, destination, commitID)
}

func checkoutCommit(ctx context.Context, repoPath, commitID string) error {
	return gitcmd.NewCommand("checkout", "--detach").AddDynamicArguments(commitID).
		WithDir(repoPath).RunWithStderr(ctx)
}

func writeDirectoryArchive(ctx context.Context, sourceDir, prefixDir string, archiveType repo_model.ArchiveType, modTime time.Time, w io.Writer) error {
	switch archiveType {
	case repo_model.ArchiveZip:
		return writeZipArchive(ctx, sourceDir, prefixDir, modTime, w)
	case repo_model.ArchiveTarGz:
		return writeTarGzArchive(ctx, sourceDir, prefixDir, modTime, w)
	default:
		return ErrUnknownArchiveFormat{RequestNameType: archiveType.String()}
	}
}

func writeZipArchive(ctx context.Context, sourceDir, prefixDir string, modTime time.Time, out io.Writer) error {
	zw := zip.NewWriter(out)
	defer zw.Close()

	// The legacy MS-DOS time field follows the time's location, which differs between servers
	modTime = modTime.UTC()
	if prefixDir != "" {
		hdr := &zip.FileHeader{
			Name:     filepath.ToSlash(prefixDir) + "/",
			Method:   zip.Store,
			Modified: modTime,
		}
		hdr.SetMode(fs.ModeDir | 0o755)
		if _, err := zw.CreateHeader(hdr); err != nil {
			return err
		}
	}

	return filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == sourceDir {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		zipPath := filepath.ToSlash(relPath)
		if prefixDir != "" {
			zipPath = filepath.ToSlash(filepath.Join(prefixDir, relPath))
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not supported in SB archive: %s", zipPath)
		}

		if d.IsDir() {
			hdr := &zip.FileHeader{
				Name:     strings.TrimSuffix(zipPath, "/") + "/",
				Method:   zip.Store,
				Modified: modTime,
			}
			hdr.SetMode(fs.ModeDir | 0o755)
			_, err := zw.CreateHeader(hdr)
			return err
		}

		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = zipPath
		hdr.Method = zip.Deflate
		hdr.Modified = modTime
		hdr.SetMode(0o644) // not the umask of the server that converted it

		writer, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

func writeTarGzArchive(ctx context.Context, sourceDir, prefixDir string, modTime time.Time, out io.Writer) error {
	gzw := gzip.NewWriter(out)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	if prefixDir != "" {
		rootHdr := &tar.Header{
			Name:     strings.TrimSuffix(filepath.ToSlash(prefixDir), "/") + "/",
			Mode:     0o755,
			Typeflag: tar.TypeDir,
			ModTime:  modTime,
		}
		if err := tw.WriteHeader(rootHdr); err != nil {
			return err
		}
	}

	return filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == sourceDir {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		archivePath := filepath.ToSlash(relPath)
		if prefixDir != "" {
			archivePath = filepath.ToSlash(filepath.Join(prefixDir, relPath))
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		linkTarget := ""
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		hdr, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		hdr.Name = archivePath
		hdr.ModTime = modTime
		// Nothing of the server that converted it: its umask, user and group
		hdr.Mode, hdr.Uid, hdr.Gid, hdr.Uname, hdr.Gname = 0o644, 0, 0, "", ""
		if d.IsDir() {
			hdr.Name = strings.TrimSuffix(hdr.Name, "/") + "/"
			hdr.Mode = 0o755
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
	})
}

func doArchive(ctx context.Context, r *ArchiveRequest) (*repo_model.RepoArchiver, error) {
	ctx, _, finished := process.GetManager().AddContext(ctx, fmt.Sprintf("SBArchiveRequest[%d]: %s", r.RepoID, r.GetArchiveName()))
	defer finished()

	archiver, err := repo_model.GetRepoArchiver(ctx, r.RepoID, r.Type, r.StorageCommitID())
	if err != nil {
		return nil, err
	}

	createdNew := false
	if archiver != nil {
		if archiver.Status == repo_model.ArchiverGenerating {
			return nil, nil //nolint:nilnil // nil archiver means it's currently being generated
		}
	} else {
		archiver = &repo_model.RepoArchiver{
			RepoID:   r.RepoID,
			Type:     r.Type,
			CommitID: r.StorageCommitID(),
			Status:   repo_model.ArchiverGenerating,
		}
		if err := db.Insert(ctx, archiver); err != nil {
			return nil, err
		}
		createdNew = true
	}

	rPath := archiver.RelativePath()
	_, err = storage.RepoArchives.Stat(rPath)
	if err == nil {
		if archiver.Status == repo_model.ArchiverGenerating {
			archiver.Status = repo_model.ArchiverReady
			if err = repo_model.UpdateRepoArchiverStatus(ctx, archiver); err != nil {
				return nil, err
			}
		}
		return archiver, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		if createdNew {
			_, _ = db.DeleteByID[repo_model.RepoArchiver](ctx, archiver.ID)
		}
		return nil, fmt.Errorf("unable to stat archive: %w", err)
	}

	repo, err := repo_model.GetRepositoryByID(ctx, archiver.RepoID)
	if err != nil {
		if createdNew {
			_, _ = db.DeleteByID[repo_model.RepoArchiver](ctx, archiver.ID)
		}
		return nil, fmt.Errorf("repo_model.GetRepositoryByID: %w", err)
	}

	rd, w := io.Pipe()
	defer func() {
		_ = w.Close()
		_ = rd.Close()
	}()

	done := make(chan error, 1)
	go func(done chan error, pw *io.PipeWriter, req *ArchiveRequest, repo *repo_model.Repository) {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("%v", r)
			}
		}()

		err := req.Stream(ctx, repo, pw)
		_ = pw.CloseWithError(err)
		done <- err
	}(done, w, r, repo)

	if _, err := storage.RepoArchives.Save(rPath, rd, -1); err != nil {
		if createdNew {
			_, _ = db.DeleteByID[repo_model.RepoArchiver](ctx, archiver.ID)
		}
		return nil, fmt.Errorf("unable to write archive: %w", err)
	}

	err = <-done
	if err != nil {
		if createdNew {
			_, _ = db.DeleteByID[repo_model.RepoArchiver](ctx, archiver.ID)
			if delErr := storage.RepoArchives.Delete(rPath); delErr != nil && !errors.Is(delErr, os.ErrNotExist) {
				log.Error("delete failed SB archive file failed: %v", delErr)
			}
		}
		return nil, err
	}

	if archiver.Status == repo_model.ArchiverGenerating {
		archiver.Status = repo_model.ArchiverReady
		if err = repo_model.UpdateRepoArchiverStatus(ctx, archiver); err != nil {
			return nil, err
		}
	}

	return archiver, nil
}

var sbArchiverQueue *queue.WorkerPoolQueue[*ArchiveRequest]

// Init initializes SB archive queue.
func Init(ctx context.Context) error {
	handler := func(items ...*ArchiveRequest) []*ArchiveRequest {
		for _, archiveReq := range items {
			log.Trace("SBArchiverData Process: %#v", archiveReq)
			if archiver, err := doArchive(ctx, archiveReq); err != nil {
				log.Error("SB archive %v failed: %v", archiveReq, err)
			} else {
				log.Trace("SBArchiverData Success: %#v", archiver)
			}
		}
		return nil
	}

	sbArchiverQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "repo-sb-archive", handler)
	if sbArchiverQueue == nil {
		return errors.New("unable to create repo-sb-archive queue")
	}
	go graceful.GetManager().RunWithCancel(sbArchiverQueue)

	return nil
}

// StartArchive pushes an SB archive request to the queue.
func StartArchive(request *ArchiveRequest) error {
	has, err := sbArchiverQueue.Has(request)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	return sbArchiverQueue.Push(request)
}

// ServeRepoSBArchive serves the generated SB archive to the client.
func ServeRepoSBArchive(ctx *gitea_context.Base, repo *repo_model.Repository, archiveReq *ArchiveRequest) {
	downloadName := repo.Name + "-" + archiveReq.GetArchiveName()
	contentType := archiveContentType(archiveReq.Type)

	cacheControl, err := sbCacheControl(ctx, repo)
	if err != nil {
		log.Error("SB archive %v: %v", archiveReq, err)
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}

	if !setting.DCS.CacheSBArchives {
		etag := sbArchiveETag(archiveReq)
		if handleNotModified(ctx, etag, cacheControl) {
			return
		}
		httplib.ServeSetHeaders(ctx.Resp, httplib.ServeHeaderOptions{Filename: downloadName, ContentType: contentType})
		ctx.Resp.Header().Set("Cache-Control", cacheControl)
		if err := archiveReq.Stream(ctx, repo, ctx.Resp); err != nil && !ctx.Written() {
			log.Error("SB archive %v streaming failed: %v", archiveReq, err)
			ctx.HTTPError(http.StatusInternalServerError)
		}
		return
	}

	archiver, err := archiveReq.Await(ctx)
	if err != nil {
		log.Error("SB archive %v await failed: %v", archiveReq, err)
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}

	rPath := archiver.RelativePath()
	if setting.RepoArchive.Storage.ServeDirect() {
		u, err := storage.RepoArchives.ServeDirectURL(rPath, downloadName, ctx.Req.Method, nil)
		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return
		}
	}

	fr, err := storage.RepoArchives.Open(rPath)
	if err != nil {
		log.Error("SB archive %v open file failed: %v", archiveReq, err)
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}
	defer fr.Close()

	httplib.ServeSetHeaders(ctx.Resp, httplib.ServeHeaderOptions{Filename: downloadName, ContentType: contentType})
	ctx.Resp.Header().Set("Cache-Control", cacheControl)
	ctx.Resp.Header().Set("ETag", sbArchiveETag(archiveReq))
	http.ServeContent(ctx.Resp, ctx.Req, downloadName, time.Time{}, fr) // no Last-Modified, see sbCacheControl
}

// sbArchiveETag names an SB archive by its cache key, which covers everything that decides its bytes, so a
// rebuild after the archive cleanup keeps the tag. It is weak because "git archive" output of an SB repo can
// change with the git version, so a resumed download (If-Range) must not join bytes of two builds.
func sbArchiveETag(aReq *ArchiveRequest) string {
	return fmt.Sprintf(`W/"%s.%s"`, aReq.StorageCommitID(), aReq.Type)
}

// sbCacheControl has every SB response revalidated with its ETag, and SB responses send no Last-Modified.
// No URL fixes everything a burrito is made from: a tag or branch can move, and a commit's metadata rows,
// TW/TWL partner commit and en_tw commit can change, while the ETag changes with any of them.
func sbCacheControl(ctx context.Context, repo *repo_model.Repository) (string, error) {
	if err := repo.LoadOwner(ctx); err != nil {
		return "", fmt.Errorf("repo.LoadOwner: %w", err)
	}
	scope := util.Iif(!repo.IsPrivate && repo.Owner.Visibility == structs.VisibleTypePublic, "public", "private")
	return "max-age=0, " + scope + ", must-revalidate", nil
}

// archiveContentType is the media type sent for an SB archive; the API's swagger "produces" lists the same ones.
func archiveContentType(t repo_model.ArchiveType) string {
	if t == repo_model.ArchiveTarGz {
		return "application/gzip"
	}
	return "application/zip"
}
