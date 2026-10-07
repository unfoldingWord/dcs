// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTcTsMetadataTypeFromRepoName(t *testing.T) {
	tests := map[string]string{
		"en_ult_gen_book":   "tc",
		"EN_ULT_GEN_BOOK":   "tc",
		"fr_gen_text_ulb":   "ts",
		"id_obs_text_obs":   "ts",
		"en_tn":             "",
		"en-textstories-x":  "",
		"book":              "",
		"something_book_v2": "",
	}
	for name, want := range tests {
		assert.Equal(t, want, GetTcTsMetadataTypeFromRepoName(name), name)
	}
}
