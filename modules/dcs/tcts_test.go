// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseTcTsManifestFlavor checks that a ts or tc repo gets the flavor of the burrito
// it is converted to by way of go-ts2rc/go-tc2rc and go-rc2sb.
func TestParseTcTsManifestFlavor(t *testing.T) {
	for manifest, want := range map[string]SBFlavor{
		`{"tc_version": 8}`: {"scripture", "textTranslation"},
		`{"package_version": 6, "type": {"id": "text"}, "project": {"id": "tit"}, "resource": {"id": "ulb"}}`: {"scripture", "textTranslation"},
		`{"package_version": 6, "type": {"id": "tn"}, "project": {"id": "tit"}, "resource": {"id": "tn"}}`:    {"parascriptural", "x-bcvnotes"},
		`{"package_version": 6, "type": {"id": "tq"}, "project": {"id": "tit"}, "resource": {"id": "tq"}}`:    {"parascriptural", "x-bcvquestions"},
		`{"package_version": 6, "type": {"id": "tw"}, "project": {"id": "bible"}, "resource": {"id": "tw"}}`:  {"parascriptural", "x-bcvarticles"},
		`{"package_version": 6, "type": {"id": "tn"}, "project": {"id": "obs"}, "resource": {"id": "obs"}}`:   {"peripheral", "x-obsnotes"},
		`{"package_version": 3, "project_id": "gen", "project": {"type": "tn"}}`:                              {"parascriptural", "x-bcvnotes"},
		`{"package_version": 1, "project_id": "obs"}`:                                                         {"gloss", "textStories"},
		`{"package_version": 3, "project": {"id": "tw"}}`:                                                     {"parascriptural", "x-bcvarticles"},
		`{"package_version": 3, "project": {"id": "ta"}}`:                                                     {"peripheral", "x-peripheralArticles"},
	} {
		ts, err := ParseTcTsManifest([]byte(manifest))
		require.NoError(t, err)
		assert.Equal(t, want, SBFlavor{ts.FlavorType, ts.Flavor}, manifest)
	}

	save, err := ParseTcTsManifest([]byte(`{"package_version": 1, "generator": {"name": "tc-desktop"}, "project": {"id": "tit"}, "target_language": {"id": "en"}}`))
	require.NoError(t, err)
	assert.Equal(t, "tc", save.MetadataType, "a translationCore save from before tc_version names tc-desktop as its generator")
	assert.Equal(t, "0", save.MetadataVersion)

	bare, err := ParseTcTsManifest([]byte(`{"package_version": 1, "generator": "tc-desktop", "project": {"id": "tit"}}`))
	require.NoError(t, err, "a generator given as a bare string still reads")
	assert.Equal(t, "ts", bare.MetadataType, "go-tc2rc only takes tc-desktop from a generator object, so a bare string is no tC save")

	v2, err := ParseTcTsManifest([]byte(`{"package_version": 2, "slug": "phm", "target_language": {"slug": "aa", "name": "Afar"}}`))
	require.NoError(t, err)
	assert.Equal(t, "aa", v2.TargetLanguage.ID, "a v2 manifest names its language by slug")
}
