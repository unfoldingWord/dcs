// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sbarchiver

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/typesniffer"
	"gitea.dev/modules/util"
	gitea_context "gitea.dev/services/context"
)

const (
	burritoMetadataFile    = "metadata.json"
	maxBurritoMetadataSize = 32 * 1024 * 1024
	sniffLen               = 1024
)

// burritoFile is one file of a burrito, found in its SB zip archive.
type burritoFile struct {
	path string
	size int64
	etag string
	open func() (io.ReadCloser, error)
	// readMetadata returns the burrito's metadata.json, which names the file's mimeType when it is an ingredient
	readMetadata func() ([]byte, error)
}

// ServeSBFile serves one file of the repo's burrito at ref (the default branch when empty), at its path
// inside the burrito: metadata.json or an ingredient. It is read out of the ref's SB zip archive, the one
// /sb/{ref}.zip serves, so the bytes are the same, including for a repo already in SB format, whose
// archive "git archive" makes with its .gitattributes export-ignore and export-subst applied.
// The returned errors are util.ErrInvalidArgument for a bad path, and util.ErrNotExist,
// RepoRefNotFoundError and ErrRepoNotConvertible for what can't be found; nothing is written then.
func ServeSBFile(ctx *gitea_context.Base, repo *repo_model.Repository, gitRepo *git.Repository, ref, filePath string) error {
	if !isValidBurritoPath(filePath) {
		return util.NewInvalidArgumentErrorf("invalid burrito file path %q", filePath)
	}
	if ref == "" {
		ref = repo.DefaultBranch
	}
	aReq, err := newRequest(ctx, repo, gitRepo, ref, repo_model.ArchiveZip)
	if err != nil {
		return err
	}
	cacheControl, err := sbCacheControl(ctx, repo)
	if err != nil {
		return err
	}

	fr, err := openSBZipArchive(ctx, repo, aReq)
	if err != nil {
		return err
	}
	defer fr.Close()
	zr, err := openZip(fr)
	if err != nil {
		return fmt.Errorf("read SB archive of %s: %w", aReq.StorageCommitID(), err)
	}
	file, err := burritoFileFromZip(zr, aReq.StorageCommitID(), filePath)
	if err != nil {
		return err
	}
	return serveBurritoFile(ctx, file, aReq, cacheControl)
}

// isValidBurritoPath rejects an empty or absolute path, a backslash, and a "." or ".." segment
func isValidBurritoPath(filePath string) bool {
	if filePath == "" || strings.HasPrefix(filePath, "/") || strings.Contains(filePath, `\`) {
		return false
	}
	for segment := range strings.SplitSeq(filePath, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// openSBZipArchive opens the kept zip archive of aReq, which is built first when the commit (or the
// TW/TWL partner commit) has none yet. Without CACHE_SB_ARCHIVES it is built into a temporary file
// for this request alone.
func openSBZipArchive(ctx context.Context, repo *repo_model.Repository, aReq *ArchiveRequest) (storage.Object, error) {
	if !setting.DCS.CacheSBArchives {
		f, cleanup, err := setting.AppDataTempDir("repo-sb-archive").CreateTempFileRandom(fmt.Sprintf("%d-*.zip", repo.ID))
		if err != nil {
			return nil, fmt.Errorf("create temporary file: %w", err)
		}
		if err := aReq.Stream(ctx, repo, f); err != nil {
			cleanup()
			return nil, err
		}
		return &tempObject{File: f, cleanup: cleanup}, nil
	}

	archiver, err := aReq.Await(ctx)
	if err != nil {
		return nil, err
	}
	fr, err := storage.RepoArchives.Open(archiver.RelativePath())
	if err != nil {
		return nil, fmt.Errorf("open SB archive %s: %w", archiver.RelativePath(), err)
	}
	return fr, nil
}

// tempObject removes its file on Close
type tempObject struct {
	*os.File
	cleanup func()
}

func (t *tempObject) Close() error {
	t.cleanup()
	return nil
}

func openZip(fr storage.Object) (*zip.Reader, error) {
	fi, err := fr.Stat()
	if err != nil {
		return nil, err
	}
	switch f := fr.(type) {
	case *os.File:
		return zip.NewReader(f, fi.Size())
	case *tempObject:
		return zip.NewReader(f.File, fi.Size())
	}
	return zip.NewReader(&streamReaderAt{rs: fr, pos: -1}, fi.Size())
}

// maxReadThrough is how far ahead a read may start and still continue the stream rather than seek
const maxReadThrough = 64 * 1024

// streamReaderAt reads a stored object for archive/zip, which makes many small reads in a row. A minio
// object's own ReadAt is a new request each time, so a read that starts at or a little after the end of
// the last one continues the open stream instead.
type streamReaderAt struct {
	rs  io.ReadSeeker
	pos int64 // -1 when unknown
}

func (s *streamReaderAt) ReadAt(p []byte, off int64) (int, error) {
	switch {
	case s.pos < 0 || off < s.pos || off-s.pos > maxReadThrough:
		if _, err := s.rs.Seek(off, io.SeekStart); err != nil {
			s.pos = -1
			return 0, err
		}
	case off > s.pos:
		if _, err := io.CopyN(io.Discard, s.rs, off-s.pos); err != nil {
			s.pos = -1
			return 0, err
		}
	}
	n, err := io.ReadFull(s.rs, p)
	s.pos = off + int64(n)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}
	return n, err
}

func burritoFileFromZip(zr *zip.Reader, storageCommitID, filePath string) (*burritoFile, error) {
	root := burritoRoot(zr)
	var file, metadata *zip.File
	for _, f := range zr.File {
		switch f.Name {
		case root + filePath:
			if !f.FileInfo().IsDir() {
				file = f
			}
		case root + burritoMetadataFile:
			metadata = f
		}
	}
	if file == nil {
		return nil, util.NewNotExistErrorf("%s is not in the burrito", filePath)
	}
	return &burritoFile{
		path: filePath,
		size: int64(file.UncompressedSize64),
		// The archive is rebuilt after the archive cleanup removes it, so the tag names the content, not the commit alone
		etag: fmt.Sprintf(`"%s-%08x-%d"`, storageCommitID, file.CRC32, file.UncompressedSize64),
		open: file.Open,
		readMetadata: func() ([]byte, error) {
			if metadata == nil {
				return nil, util.NewNotExistErrorf("the burrito has no %s", burritoMetadataFile)
			}
			rc, err := metadata.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, maxBurritoMetadataSize))
		},
	}, nil
}

// burritoRoot returns the folder that holds every file of the archive, with a trailing slash, or "" when
// the archive has files at its top level (PREFIX_ARCHIVE_FILES = false). It is read from the archive
// rather than made from the repo name, since a repo renamed after the archive was built keeps its old
// name there, and "git archive" lowercases it.
func burritoRoot(zr *zip.Reader) string {
	root := ""
	for _, f := range zr.File {
		folder, _, found := strings.Cut(f.Name, "/")
		if !found || root != "" && folder+"/" != root {
			return ""
		}
		root = folder + "/"
	}
	if root == "" || setting.Repository.PrefixArchiveFiles {
		return root
	}
	// An unprefixed archive whose .gitattributes export only one folder looks prefixed too, so it is a prefix
	// only when the burrito's metadata.json is right under it
	for _, f := range zr.File {
		if f.Name == root+burritoMetadataFile {
			return root
		}
	}
	return ""
}

// handleNotModified sets the cache headers and answers 304 when If-None-Match names the current ETag.
// If-Modified-Since isn't answered, since SB responses have no Last-Modified (see sbCacheControl).
func handleNotModified(ctx *gitea_context.Base, etag, cacheControl string) bool {
	header := ctx.Resp.Header()
	header.Set("ETag", etag)
	header.Set("Cache-Control", cacheControl)
	for tag := range strings.SplitSeq(ctx.Req.Header.Get("If-None-Match"), ",") {
		tag = strings.TrimPrefix(strings.TrimSpace(tag), "W/")
		if tag == "*" || tag != "" && tag == strings.TrimPrefix(etag, "W/") {
			ctx.Resp.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	return false
}

func serveBurritoFile(ctx *gitea_context.Base, file *burritoFile, aReq *ArchiveRequest, cacheControl string) error {
	if handleNotModified(ctx, file.etag, cacheControl) {
		return nil
	}

	contentType := "application/json"
	if file.path != burritoMetadataFile {
		contentType = ingredientContentType(file)
	}

	rc, err := file.open()
	if err != nil {
		return err
	}
	defer rc.Close()
	br := bufio.NewReaderSize(rc, sniffLen)
	if contentType == "" {
		head, _ := br.Peek(sniffLen)
		contentType = sniffContentType(head)
	}

	httplib.ServeSetHeaders(ctx.Resp, httplib.ServeHeaderOptions{
		ContentType:        contentType,
		ContentLength:      &file.size,
		Filename:           path.Base(file.path),
		ContentDisposition: httplib.ContentDispositionInline,
	})
	ctx.Resp.Header().Set("Cache-Control", cacheControl) // ServeSetHeaders sets its own
	ctx.Resp.WriteHeader(http.StatusOK)
	if _, err := io.Copy(ctx.Resp, br); err != nil {
		log.Debug("SB file %s of %s: %v", file.path, aReq.StorageCommitID(), err) // the status is sent, so it can't be reported
	}
	return nil
}

// ingredientContentType returns the file's mimeType from metadata.json, or "" when it isn't an ingredient.
// HTML is sent as text, as /raw does, so that a burrito can't put a page on this site.
func ingredientContentType(file *burritoFile) string {
	content, err := file.readMetadata()
	if err != nil {
		return ""
	}
	var metadata struct {
		Ingredients map[string]struct {
			MimeType string `json:"mimeType"`
		} `json:"ingredients"`
	}
	if err := json.Unmarshal(content, &metadata); err != nil {
		return ""
	}
	mimeType := metadata.Ingredients[file.path].MimeType
	mediaType, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		return ""
	}
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		return "text/plain"
	}
	return mimeType
}

// sniffContentType types a file that metadata.json doesn't list, as /raw does
func sniffContentType(head []byte) string {
	sniffed := typesniffer.DetectContentType(head)
	switch {
	case sniffed.IsBrowsableBinaryType():
		return sniffed.GetMimeType()
	case sniffed.IsText():
		if encoding, _ := charset.DetectEncoding(head); encoding != "" {
			return "text/plain; charset=" + strings.ToLower(encoding)
		}
		return "text/plain"
	}
	return typesniffer.MimeTypeApplicationOctetStream
}
