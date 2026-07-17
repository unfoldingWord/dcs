// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sbarchiver

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	"gitea.dev/modules/process"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/util"
	gitea_context "gitea.dev/services/context"

	rc2sb "github.com/unfoldingWord/go-rc2sb"
	tc2rc "github.com/unfoldingWord/go-tc2rc"
	ts2rc "github.com/unfoldingWord/go-ts2rc"
)

const storageCommitPrefix = "sb-"

// ArchiveRequest defines the parameters of an SB archive request.
type ArchiveRequest struct {
	RepoID   int64
	Type     repo_model.ArchiveType
	CommitID string

	archiveRefShortName string // "master", "v1.0.0", commit id, etc.
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

// ErrRepoNotConvertible means SB archive generation is only available for RC and ts repositories.
type ErrRepoNotConvertible struct {
	RepoID int64
}

func (e ErrRepoNotConvertible) Error() string {
	return fmt.Sprintf("repository %d is not an RC or ts repository", e.RepoID)
}

func (e ErrRepoNotConvertible) Is(err error) bool {
	_, ok := err.(ErrRepoNotConvertible)
	return ok
}

// NewRequest creates an SB archival request from URI suffix like "master.zip".
func NewRequest(repoID int64, repo *git.Repository, archiveRefExt string) (*ArchiveRequest, error) {
	archiveRefShortName, archiveType := repo_model.SplitArchiveNameType(archiveRefExt)
	if archiveType == repo_model.ArchiveUnknown {
		return nil, ErrUnknownArchiveFormat{archiveRefExt}
	}
	if archiveType == repo_model.ArchiveBundle {
		return nil, ErrUnknownArchiveFormat{archiveRefExt}
	}

	commitID, err := repo.ConvertToGitID(archiveRefShortName)
	if err != nil {
		return nil, RepoRefNotFoundError{RefShortName: archiveRefShortName}
	}

	return &ArchiveRequest{
		RepoID:              repoID,
		Type:                archiveType,
		CommitID:            commitID.String(),
		archiveRefShortName: archiveRefShortName,
	}, nil
}

// GetArchiveName returns archive name based on requested ref.
func (aReq *ArchiveRequest) GetArchiveName() string {
	return strings.ReplaceAll(aReq.archiveRefShortName, "/", "-") + "-sb." + aReq.Type.String()
}

// StorageCommitID returns the cache key commit ID used in repo_archiver.
func (aReq *ArchiveRequest) StorageCommitID() string {
	return storageCommitPrefix + aReq.CommitID
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

// Stream generates and streams a converted SB archive.
func (aReq *ArchiveRequest) Stream(ctx context.Context, repo *repo_model.Repository, w io.Writer) error {
	repoDM, err := getRepoDMForConversion(ctx, repo)
	if err != nil {
		return err
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
	if err := cloneRepositoryAtCommit(ctx, repo.RepoPath(), aReq.archiveRefShortName, aReq.CommitID, cloneDir); err != nil {
		return fmt.Errorf("clone requested repo ref: %w", err)
	}

	// For ts repos, convert ts -> RC so the RC -> SB pipeline below applies unchanged.
	if isTS {
		twSourceDir, err := prepareTsTWSourceDir(ctx, tmpDir, repoDM)
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

	payloadPath, err := preparePayloadPath(ctx, tmpDir, rcDir, repo, repoDM)
	if err != nil {
		return err
	}

	sbDir := filepath.Join(tmpDir, "sb")
	_, err = rc2sb.Convert(ctx, rcDir, sbDir, rc2sb.Options{
		PayloadPath: payloadPath,
	})
	if err != nil {
		return fmt.Errorf("rc2sb.Convert: %w", err)
	}

	prefixDir := ""
	if setting.Repository.PrefixArchiveFiles {
		prefixDir = repo.Name
	}
	if err := writeDirectoryArchive(ctx, sbDir, prefixDir, aReq.Type, w); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}

	return nil
}

func getRepoDMForConversion(ctx context.Context, repo *repo_model.Repository) (*repo_model.Door43Metadata, error) {
	if err := repo.LoadLatestDMs(ctx); err != nil {
		return nil, fmt.Errorf("repo.LoadLatestDMs: %w", err)
	}

	dm := repo.DefaultBranchDM
	if dm == nil {
		dm = repo.RepoDM
	}
	if dm == nil || (dm.MetadataType != "rc" && dm.MetadataType != "ts" && dm.MetadataType != "tc") {
		return nil, ErrRepoNotConvertible{RepoID: repo.ID}
	}
	return dm, nil
}

// tsTWSourceOwners are tried in order when locating a canonical en_tw repo. ts2rc uses it
// to place Translation Words articles under bible/{kt|names|other}/ by matching slug filenames.
var tsTWSourceOwners = []string{"unfoldingWord", "Door43-Catalog"}

// prepareTsTWSourceDir clones a canonical en_tw repo into tmpDir/tw-source for ts
// Translation Words conversions and returns its path. Returns ("", nil) when the repo
// is not a ts TW repo or no canonical en_tw repo exists on this server — ts2rc then
// falls back to writing articles under bible/other/ with a warning.
func prepareTsTWSourceDir(ctx context.Context, tmpDir string, dm *repo_model.Door43Metadata) (string, error) {
	if dm == nil || dm.Subject != "Translation Words" {
		return "", nil
	}

	for _, owner := range tsTWSourceOwners {
		twRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, owner, "en_tw")
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				continue
			}
			return "", fmt.Errorf("repo_model.GetRepositoryByOwnerAndName(%s/en_tw): %w", owner, err)
		}

		twGitRepo, err := gitrepo.OpenRepository(ctx, twRepo)
		if err != nil {
			return "", fmt.Errorf("gitrepo.OpenRepository(%s): %w", twRepo.FullName(), err)
		}
		commitID, err := twGitRepo.ConvertToGitID(twRepo.DefaultBranch)
		twGitRepo.Close()
		if err != nil {
			return "", fmt.Errorf("resolve %s default branch: %w", twRepo.FullName(), err)
		}

		twSourceDir := filepath.Join(tmpDir, "tw-source")
		if err := cloneRepositoryAtCommit(ctx, twRepo.RepoPath(), twRepo.DefaultBranch, commitID.String(), twSourceDir); err != nil {
			return "", fmt.Errorf("clone TW source repo %s: %w", twRepo.FullName(), err)
		}
		return twSourceDir, nil
	}

	log.Warn("SB archive: no canonical en_tw repo found for TW category lookup; articles will fall back to bible/other/")
	return "", nil
}

func preparePayloadPath(ctx context.Context, tmpDir, rcDir string, repo *repo_model.Repository, dm *repo_model.Door43Metadata) (string, error) {
	if dm.Language == "" {
		return "", nil
	}

	var payloadRepoName string
	isTWRepo := false
	switch dm.Subject {
	case "TSV Translation Words Links":
		payloadRepoName = dm.Language + "_tw"
	case "Translation Words":
		payloadRepoName = dm.Language + "_twl"
		isTWRepo = true
	default:
		return "", nil
	}

	payloadRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, repo.OwnerName, payloadRepoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("repo_model.GetRepositoryByOwnerAndName(%s): %w", payloadRepoName, err)
	}

	payloadGitRepo, err := gitrepo.OpenRepository(ctx, payloadRepo)
	if err != nil {
		return "", fmt.Errorf("gitrepo.OpenRepository(%s): %w", payloadRepo.FullName(), err)
	}
	defer payloadGitRepo.Close()

	commitID, err := payloadGitRepo.ConvertToGitID(payloadRepo.DefaultBranch)
	if err != nil {
		return "", fmt.Errorf("resolve %s default branch: %w", payloadRepo.FullName(), err)
	}

	if isTWRepo {
		// Clone the TWL repo directly into rcDir/<lang>_twl/ so the library auto-detects
		// it from inDir (the same pattern the TWL handler uses to auto-detect <lang>_tw/).
		destDir := filepath.Join(rcDir, payloadRepoName)
		if err := cloneRepositoryAtCommit(ctx, payloadRepo.RepoPath(), payloadRepo.DefaultBranch, commitID.String(), destDir); err != nil {
			return "", fmt.Errorf("clone TWL payload into rcDir: %w", err)
		}
		return "", nil
	}

	payloadDir := filepath.Join(tmpDir, "payload")
	if err := cloneRepositoryAtCommit(ctx, payloadRepo.RepoPath(), payloadRepo.DefaultBranch, commitID.String(), payloadDir); err != nil {
		return "", fmt.Errorf("clone payload repository %s: %w", payloadRepo.FullName(), err)
	}

	return payloadDir, nil
}

func cloneRepositoryAtCommit(ctx context.Context, sourceRepoPath, refShortName, commitID, destination string) error {
	tryShallow := refShortName != "" && commitID != ""
	if tryShallow {
		err := git.Clone(ctx, sourceRepoPath, destination, git.CloneRepoOptions{
			Quiet:  true,
			Depth:  1,
			Branch: refShortName,
		})
		if err == nil {
			return checkoutCommit(ctx, destination, commitID)
		}
		if removeErr := util.RemoveAll(destination); removeErr != nil {
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

func writeDirectoryArchive(ctx context.Context, sourceDir, prefixDir string, archiveType repo_model.ArchiveType, w io.Writer) error {
	switch archiveType {
	case repo_model.ArchiveZip:
		return writeZipArchive(ctx, sourceDir, prefixDir, w)
	case repo_model.ArchiveTarGz:
		return writeTarGzArchive(ctx, sourceDir, prefixDir, w)
	default:
		return ErrUnknownArchiveFormat{RequestNameType: archiveType.String()}
	}
}

func writeZipArchive(ctx context.Context, sourceDir, prefixDir string, out io.Writer) error {
	zw := zip.NewWriter(out)
	defer zw.Close()

	if prefixDir != "" {
		hdr := &zip.FileHeader{
			Name:     filepath.ToSlash(prefixDir) + "/",
			Method:   zip.Store,
			Modified: time.Now(),
		}
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
				Modified: info.ModTime(),
			}
			_, err := zw.CreateHeader(hdr)
			return err
		}

		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = zipPath
		hdr.Method = zip.Deflate

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

func writeTarGzArchive(ctx context.Context, sourceDir, prefixDir string, out io.Writer) error {
	gzw := gzip.NewWriter(out)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	if prefixDir != "" {
		rootHdr := &tar.Header{
			Name:     strings.TrimSuffix(filepath.ToSlash(prefixDir), "/") + "/",
			Mode:     0o755,
			Typeflag: tar.TypeDir,
			ModTime:  time.Now(),
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
		if d.IsDir() {
			hdr.Name = strings.TrimSuffix(hdr.Name, "/") + "/"
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
	ctx.Resp.Header().Add("Link", fmt.Sprintf(`<%s/sb/%s.%s?rev=%s>; rel="immutable"`,
		repo.APIURL(),
		archiveReq.CommitID,
		archiveReq.Type.String(),
		archiveReq.CommitID,
	))
	downloadName := repo.Name + "-" + archiveReq.GetArchiveName()

	if setting.Repository.StreamArchives {
		httplib.ServeSetHeaders(ctx.Resp, httplib.ServeHeaderOptions{Filename: downloadName})
		if err := archiveReq.Stream(ctx, repo, ctx.Resp); err != nil && !ctx.Written() {
			if errors.Is(err, ErrRepoNotConvertible{}) {
				ctx.HTTPError(http.StatusNotFound)
				return
			}
			log.Error("SB archive %v streaming failed: %v", archiveReq, err)
			ctx.HTTPError(http.StatusInternalServerError)
		}
		return
	}

	archiver, err := archiveReq.Await(ctx)
	if err != nil {
		if errors.Is(err, ErrRepoNotConvertible{}) {
			ctx.HTTPError(http.StatusNotFound)
			return
		}
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

	ctx.ServeContent(fr, gitea_context.ServeHeaderOptions{
		Filename:     downloadName,
		LastModified: archiver.CreatedUnix.AsLocalTime(),
	})
}
