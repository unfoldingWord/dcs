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
	"time"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/json"
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

// burritoFile is one file of a burrito, found in the SB archive or, for a repo already in SB format, in git.
type burritoFile struct {
	path string
	size int64
	etag string
	open func() (io.ReadCloser, error)
	// readMetadata returns the burrito's metadata.json, which names the file's mimeType when it is an ingredient
	readMetadata func() ([]byte, error)
}

// ServeSBFile serves one file of the repo's burrito at ref (the default branch when empty), at its path
// inside the burrito: metadata.json or an ingredient. The bytes are those of the same path in the ref's
// SB zip archive, read from the cached archive; a repo already in SB format is read from git, as /raw does.
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
	cacheControl, err := aReq.cacheControl(ctx, repo)
	if err != nil {
		return err
	}

	if aReq.dm.MetadataType == "sb" {
		file, err := burritoFileFromGit(ctx, gitRepo, aReq.CommitID, filePath)
		if err != nil {
			return err
		}
		return serveBurritoFile(ctx, file, aReq, cacheControl)
	}

	archiver, err := aReq.Await(ctx)
	if err != nil {
		return err
	}
	fr, err := storage.RepoArchives.Open(archiver.RelativePath())
	if err != nil {
		return fmt.Errorf("open SB archive: %w", err)
	}
	defer fr.Close()
	zr, err := openZip(fr)
	if err != nil {
		return fmt.Errorf("read SB archive %s: %w", archiver.RelativePath(), err)
	}
	file, err := burritoFileFromZip(zr, aReq.StorageCommitID(), filePath)
	if err != nil {
		return err
	}
	return serveBurritoFile(ctx, file, aReq, cacheControl)
}

// isValidBurritoPath rejects an empty or absolute path and one that climbs out of the burrito
func isValidBurritoPath(filePath string) bool {
	if filePath == "" || strings.HasPrefix(filePath, "/") {
		return false
	}
	for segment := range strings.SplitSeq(filePath, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}

func burritoFileFromGit(ctx context.Context, gitRepo *git.Repository, commitID, filePath string) (*burritoFile, error) {
	commit, err := gitRepo.GetCommit(ctx, commitID)
	if err != nil {
		return nil, fmt.Errorf("gitRepo.GetCommit(%s): %w", commitID, err)
	}
	blob, err := commit.GetBlobByPath(ctx, gitRepo, filePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, util.NewNotExistErrorf("%s is not in the burrito", filePath)
		}
		return nil, err
	}
	return &burritoFile{
		path: filePath,
		size: blob.Size(ctx),
		etag: `"` + blob.ID.String() + `"`, // as /raw
		open: func() (io.ReadCloser, error) { return blob.DataAsync(ctx) },
		readMetadata: func() ([]byte, error) {
			metadataBlob, err := commit.GetBlobByPath(ctx, gitRepo, burritoMetadataFile)
			if err != nil {
				return nil, err
			}
			return metadataBlob.GetBlobBytes(ctx, maxBurritoMetadataSize)
		},
	}, nil
}

func openZip(fr storage.Object) (*zip.Reader, error) {
	fi, err := fr.Stat()
	if err != nil {
		return nil, err
	}
	if f, ok := fr.(*os.File); ok {
		return zip.NewReader(f, fi.Size())
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

// burritoRoot returns the archive folder holding metadata.json, with a trailing slash: the repo-named
// folder, or "" when archive files have no prefix. A repo renamed since the archive was built keeps its old
// folder name there, so the folder is looked up rather than derived from the repo name.
func burritoRoot(zr *zip.Reader) string {
	root := ""
	for _, f := range zr.File {
		if f.Name == burritoMetadataFile {
			return ""
		}
		if dir, ok := strings.CutSuffix(f.Name, "/"+burritoMetadataFile); ok && root == "" && !strings.Contains(dir, "/") {
			root = dir + "/"
		}
	}
	return root
}

// handleNotModified sets the cache headers and answers a conditional request whose copy is current.
// If-Modified-Since only counts without If-None-Match (RFC 9110 13.2.2), since a converted burrito's
// partner content can change while its commit time doesn't.
func handleNotModified(ctx *gitea_context.Base, etag string, lastModified time.Time, cacheControl string) bool {
	header := ctx.Resp.Header()
	header.Set("ETag", etag)
	header.Set("Cache-Control", cacheControl)
	if !lastModified.IsZero() {
		header.Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	}

	notModified := false
	if ifNoneMatch := ctx.Req.Header.Get("If-None-Match"); ifNoneMatch != "" {
		for tag := range strings.SplitSeq(ifNoneMatch, ",") {
			tag = strings.TrimPrefix(strings.TrimSpace(tag), "W/")
			if tag == "*" || tag == strings.TrimPrefix(etag, "W/") {
				notModified = true
				break
			}
		}
	} else if since, err := http.ParseTime(ctx.Req.Header.Get("If-Modified-Since")); err == nil && !lastModified.IsZero() {
		notModified = !lastModified.Truncate(time.Second).After(since)
	}
	if notModified {
		ctx.Resp.WriteHeader(http.StatusNotModified)
	}
	return notModified
}

func serveBurritoFile(ctx *gitea_context.Base, file *burritoFile, aReq *ArchiveRequest, cacheControl string) error {
	if handleNotModified(ctx, file.etag, aReq.commitTime, cacheControl) {
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
		LastModified:       aReq.commitTime,
	})
	ctx.Resp.Header().Set("Cache-Control", cacheControl) // ServeSetHeaders sets its own
	ctx.Resp.WriteHeader(http.StatusOK)
	_, _ = io.Copy(ctx.Resp, br) // the status is sent, so there's nothing left to report an error with
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
