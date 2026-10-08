// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"slices"
	"testing"

	"gitea.dev/models/door43metadata"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every builder field must be a prefix the page's q parser understands, or the
// builder would write tokens the search silently treats as free text.
func TestSearchBuilderFields(t *testing.T) {
	setting.SetupGiteaTestEnv()

	names := func(fields []SearchBuilderField) []string {
		out := make([]string, 0, len(fields))
		for _, f := range fields {
			out = append(out, f.Name)
		}
		return out
	}
	repoFields := SearchBuilderFields(false)
	catalogFields := SearchBuilderFields(true)
	assert.Subset(t, door43metadata.RepoSearchKeywordFields, names(repoFields))
	assert.Subset(t, door43metadata.CatalogSearchKeywordFields, names(catalogFields))
	assert.Equal(t, names(repoFields), names(catalogFields)[:len(repoFields)], "catalog fields start with the repo fields")
	assert.Contains(t, names(catalogFields), "stage")
	assert.NotContains(t, names(repoFields), "stage")

	byName := make(map[string]SearchBuilderField, len(catalogFields))
	for _, f := range catalogFields {
		byName[f.Name] = f
	}
	require.Contains(t, byName["subject"].Options, "Aligned Bible", "subjects come from the RC schema enum")
	assert.True(t, slices.IsSorted(byName["subject"].Options))
	assert.Equal(t, []string{"bak", "frt", "obs", "gen", "exo"}, byName["book"].Options[:5], "books in canonical order, unnumbered ones first")
	assert.Len(t, byName["book"].Options, len(BookNames))
	assert.Contains(t, byName["abbreviation"].Options, "ult")
	assert.Equal(t, []string{"prod", "preprod", "latest", "other"}, byName["stage"].Options)
	assert.Empty(t, byName["lang"].Options)
}
