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

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidBurritoPath(t *testing.T) {
	for _, p := range []string{"metadata.json", "ingredients/TIT.tsv", "ingredients/payload/kt/god.md", "ingredients/..x.md"} {
		assert.True(t, isValidBurritoPath(p), p)
	}
	for _, p := range []string{"", "/metadata.json", "..", "../en_tw/metadata.json", "ingredients/../../x", `ingredients\..\x`, "ingredients/./TIT.tsv", "ingredients//TIT.tsv", "ingredients/"} {
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

	// Without PREFIX_ARCHIVE_FILES there is no root folder, whatever folders the archive has: .gitattributes can
	// cut an archive down to one folder, which can even hold a metadata.json of its own
	defer test.MockVariableValue(&setting.Repository.PrefixArchiveFiles, false)()
	for _, names := range [][]string{
		{"ingredients/", "ingredients/TIT.tsv", "ingredients/metadata.json", "metadata.json"},
		{"ingredients/", "ingredients/TIT.tsv"},
		{"ingredients/", "ingredients/TIT.tsv", "ingredients/metadata.json"},
	} {
		zr = makeZip(t, map[string]string{"ingredients/TIT.tsv": "x", "metadata.json": metadata, "ingredients/metadata.json": "{}"}, names...)
		assert.Empty(t, burritoRoot(zr), names)
		file, err = burritoFileFromZip(zr, "abc123", "ingredients/TIT.tsv")
		require.NoError(t, err, names)
		assert.EqualValues(t, 1, file.size, names)
	}
}

func TestSniffContentType(t *testing.T) {
	assert.Contains(t, sniffContentType([]byte("# Notes\n")), "text/plain")
	assert.Equal(t, "image/png", sniffContentType([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")))
	assert.Equal(t, "application/octet-stream", sniffContentType([]byte{0, 1, 2, 3, 0xff}))
}

func TestHandleNotModified(t *testing.T) {
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
		// SB responses have no Last-Modified, so a date alone never keeps a copy that may be stale
		{"If-Modified-Since", time.Now().Format(http.TimeFormat), false},
	} {
		ctx, resp := contexttest.MockContext(t, "user27/repo49/sb/raw/metadata.json")
		if c.header != "" {
			ctx.Req.Header.Set(c.header, c.value)
		}
		assert.Equal(t, c.want, handleNotModified(ctx.Base, etag, "max-age=0"), "%s: %s", c.header, c.value)
		assert.Equal(t, etag, resp.Header().Get("ETag"))
		assert.Equal(t, "max-age=0", resp.Header().Get("Cache-Control"))
		assert.Empty(t, resp.Header().Get("Last-Modified"))
		if c.want {
			assert.Equal(t, http.StatusNotModified, resp.Code)
		}
	}
}
