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

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	gitea_context "code.gitea.io/gitea/services/context"

	rc2sb "github.com/unfoldingWord/go-rc2sb"
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

// ErrRepoNotRC means SB archive generation is only available for RC repositories.
type ErrRepoNotRC struct {
	RepoID int64
}

func (e ErrRepoNotRC) Error() string {
	return fmt.Sprintf("repository %d is not an RC repository", e.RepoID)
}

func (e ErrRepoNotRC) Is(err error) bool {
	_, ok := err.(ErrRepoNotRC)
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

	rcDir := filepath.Join(tmpDir, "rc")
	if err := cloneRepositoryAtCommit(ctx, repo.RepoPath(), aReq.archiveRefShortName, aReq.CommitID, rcDir); err != nil {
		return fmt.Errorf("clone requested repo ref: %w", err)
	}

	payloadPath, err := preparePayloadPath(ctx, tmpDir, repo, repoDM)
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
	if dm == nil || dm.MetadataType != "rc" {
		return nil, ErrRepoNotRC{RepoID: repo.ID}
	}
	return dm, nil
}

func preparePayloadPath(ctx context.Context, tmpDir string, repo *repo_model.Repository, dm *repo_model.Door43Metadata) (string, error) {
	if dm.Subject != "TSV Translation Words Links" || dm.Language == "" {
		return "", nil
	}

	twRepoName := dm.Language + "_tw"
	twRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, repo.OwnerName, twRepoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("repo_model.GetRepositoryByOwnerAndName(%s): %w", twRepoName, err)
	}

	twGitRepo, err := gitrepo.OpenRepository(ctx, twRepo)
	if err != nil {
		return "", fmt.Errorf("gitrepo.OpenRepository(%s): %w", twRepo.FullName(), err)
	}
	defer twGitRepo.Close()

	commitID, err := twGitRepo.ConvertToGitID(twRepo.DefaultBranch)
	if err != nil {
		return "", fmt.Errorf("resolve TW default branch %s: %w", twRepo.DefaultBranch, err)
	}

	payloadDir := filepath.Join(tmpDir, "payload")
	if err := cloneRepositoryAtCommit(ctx, twRepo.RepoPath(), twRepo.DefaultBranch, commitID.String(), payloadDir); err != nil {
		return "", fmt.Errorf("clone TW payload repository: %w", err)
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
	err := gitcmd.NewCommand("checkout", "--detach").AddDynamicArguments(commitID).
		WithDir(repoPath).
		Run(ctx)
	if err != nil {
		return fmt.Errorf("git checkout --detach %s in %s: %w", commitID, repoPath, err)
	}
	return nil
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
		httplib.ServeSetHeaders(ctx.Resp, &httplib.ServeHeaderOptions{Filename: downloadName})
		if err := archiveReq.Stream(ctx, repo, ctx.Resp); err != nil && !ctx.Written() {
			if errors.Is(err, ErrRepoNotRC{}) {
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
		if errors.Is(err, ErrRepoNotRC{}) {
			ctx.HTTPError(http.StatusNotFound)
			return
		}
		log.Error("SB archive %v await failed: %v", archiveReq, err)
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}

	rPath := archiver.RelativePath()
	if setting.RepoArchive.Storage.ServeDirect() {
		u, err := storage.RepoArchives.URL(rPath, downloadName, ctx.Req.Method, nil)
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

	ctx.ServeContent(fr, &gitea_context.ServeHeaderOptions{
		Filename:     downloadName,
		LastModified: archiver.CreatedUnix.AsLocalTime(),
	})
}
