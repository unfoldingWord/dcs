// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sbarchiver

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"gitea.dev/modules/util"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidBurritoPath(t *testing.T) {
	for _, p := range []string{"metadata.json", "ingredients/TIT.tsv", "ingredients/payload/kt/god.md", "ingredients/..x.md"} {
		assert.True(t, isValidBurritoPath(p), p)
	}
	for _, p := range []string{"", "/metadata.json", "..", "../en_tw/metadata.json", "ingredients/../../x", `ingredients\..\x`, "ingredients/./TIT.tsv"} {
		assert.False(t, isValidBurritoPath(p), p)
	}
}

func makeZip(t *testing.T, files map[string]string, names ...string) *zip.Reader {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(w, files[name])
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	return zr
}

func TestBurritoFileFromZip(t *testing.T) {
	metadata := `{"ingredients": {
		"ingredients/TIT.tsv": {"mimeType": "text/tab-separated-values"},
		"ingredients/page.html": {"mimeType": "text/html; charset=utf-8"}
	}}`
	files := map[string]string{
		"en_tn/metadata.json":             metadata,
		"en_tn/ingredients/TIT.tsv":       "Reference\tNote\n",
		"en_tn/ingredients/page.html":     "<script></script>",
		"en_tn/ingredients/metadata.json": "{}", // deeper, so not the burrito's
		"en_tn/README.md":                 "# Notes\n",
	}
	zr := makeZip(t, files, "en_tn/", "en_tn/README.md", "en_tn/ingredients/", "en_tn/ingredients/TIT.tsv",
		"en_tn/ingredients/metadata.json", "en_tn/ingredients/page.html", "en_tn/metadata.json")
	assert.Equal(t, "en_tn/", burritoRoot(zr))

	file, err := burritoFileFromZip(zr, "abc123", "ingredients/TIT.tsv")
	require.NoError(t, err)
	assert.EqualValues(t, len(files["en_tn/ingredients/TIT.tsv"]), file.size)
	assert.Regexp(t, `^"abc123-[0-9a-f]{8}-15"$`, file.etag)
	rc, err := file.open()
	require.NoError(t, err)
	content, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, files["en_tn/ingredients/TIT.tsv"], string(content))
	assert.Equal(t, "text/tab-separated-values", ingredientContentType(file))

	file, err = burritoFileFromZip(zr, "abc123", "ingredients/page.html")
	require.NoError(t, err)
	assert.Equal(t, "text/plain", ingredientContentType(file))

	file, err = burritoFileFromZip(zr, "abc123", "README.md")
	require.NoError(t, err)
	assert.Empty(t, ingredientContentType(file), "not an ingredient")

	for _, p := range []string{"ingredients", "ingredients/GEN.tsv", "en_tn/metadata.json"} {
		_, err = burritoFileFromZip(zr, "abc123", p)
		assert.ErrorIs(t, err, util.ErrNotExist, p)
	}

	// Archives without a root folder (PREFIX_ARCHIVE_FILES = false)
	files = map[string]string{"metadata.json": metadata, "ingredients/TIT.tsv": "x", "ingredients/metadata.json": "{}"}
	zr = makeZip(t, files, "ingredients/", "ingredients/TIT.tsv", "ingredients/metadata.json", "metadata.json")
	assert.Empty(t, burritoRoot(zr))
	file, err = burritoFileFromZip(zr, "abc123", "metadata.json")
	require.NoError(t, err)
	assert.EqualValues(t, len(metadata), file.size)
}

func TestSniffContentType(t *testing.T) {
	assert.Contains(t, sniffContentType([]byte("# Notes\n")), "text/plain")
	assert.Equal(t, "image/png", sniffContentType([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")))
	assert.Equal(t, "application/octet-stream", sniffContentType([]byte{0, 1, 2, 3, 0xff}))
}

func TestHandleNotModified(t *testing.T) {
	lastModified := time.Date(2026, 10, 22, 20, 6, 43, 0, time.UTC)
	const etag = `"abc-1"`
	for _, c := range []struct {
		header, value string
		want          bool
	}{
		{"", "", false},
		{"If-None-Match", `"abc-1"`, true},
		{"If-None-Match", `W/"abc-1"`, true},
		{"If-None-Match", `"other", "abc-1"`, true},
		{"If-None-Match", `*`, true},
		{"If-None-Match", `"abc-0"`, false},
		{"If-Modified-Since", lastModified.Format(http.TimeFormat), true},
		{"If-Modified-Since", lastModified.Add(-time.Second).Format(http.TimeFormat), false},
	} {
		ctx, resp := contexttest.MockContext(t, "user27/repo49/sb/raw/metadata.json")
		if c.header != "" {
			ctx.Req.Header.Set(c.header, c.value)
		}
		assert.Equal(t, c.want, handleNotModified(ctx.Base, etag, lastModified, "max-age=0"), "%s: %s", c.header, c.value)
		assert.Equal(t, etag, resp.Header().Get("ETag"))
		assert.Equal(t, "max-age=0", resp.Header().Get("Cache-Control"))
		if c.want {
			assert.Equal(t, http.StatusNotModified, resp.Code)
		}
	}

	// A stale ETag outweighs a current date: the burrito's partner content may have changed since
	ctx, _ := contexttest.MockContext(t, "user27/repo49/sb/raw/metadata.json")
	ctx.Req.Header.Set("If-None-Match", `"abc-0"`)
	ctx.Req.Header.Set("If-Modified-Since", lastModified.Format(http.TimeFormat))
	assert.False(t, handleNotModified(ctx.Base, etag, lastModified, "max-age=0"))

	// A burrito with partner content has no Last-Modified, so a date alone can't keep a stale copy
	aReq := &ArchiveRequest{commitTime: lastModified, refIsCommitID: true, ownMetadata: true, mayHavePartner: true, PayloadCommitID: "0123456789abcdef0123456789abcdef01234567"}
	ctx, resp := contexttest.MockContext(t, "user27/repo49/sb/raw/metadata.json")
	ctx.Req.Header.Set("If-Modified-Since", lastModified.Format(http.TimeFormat))
	assert.False(t, handleNotModified(ctx.Base, etag, aReq.lastModified(), "max-age=0"))
	assert.Empty(t, resp.Header().Get("Last-Modified"))
	aReq.PayloadCommitID, aReq.mayHavePartner = "", false
	assert.Equal(t, lastModified, aReq.lastModified())

	// A branch or tag can be moved to an older commit, so it has no Last-Modified either
	aReq.refIsCommitID = false
	ctx, resp = contexttest.MockContext(t, "user27/repo49/sb/raw/metadata.json")
	ctx.Req.Header.Set("If-Modified-Since", lastModified.Format(http.TimeFormat))
	assert.False(t, handleNotModified(ctx.Base, etag, aReq.lastModified(), "max-age=0"))
	assert.Empty(t, resp.Header().Get("Last-Modified"))
}
