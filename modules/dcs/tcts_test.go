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
	} {
		ts, err := ParseTcTsManifest([]byte(manifest))
		require.NoError(t, err)
		assert.Equal(t, want, SBFlavor{ts.FlavorType, ts.Flavor}, manifest)
	}
}
