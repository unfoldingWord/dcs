// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYAMLKeepsTimestampsAsText(t *testing.T) {
	doc, err := ParseYAML([]byte("issued: 2024-01-01\nmodified: 2024-01-01 10:20:30\nversions:\n  2024-02-03: x\n"))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"issued":   "2024-01-01",
		"modified": "2024-01-01 10:20:30",
		"versions": map[string]any{"2024-02-03": "x"},
	}, doc)

	doc, err = ParseYAML(nil)
	require.NoError(t, err)
	assert.Nil(t, doc, "an empty file must stay nil so the schema reports it as empty")
}
