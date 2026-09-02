// Copyright 2026 unfoldingWord. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

func TestBuildBooleanFullTextTerm(t *testing.T) {
	cases := []struct {
		name    string
		keyword string
		want    string
	}{
		{"single word gets a prefix wildcard", "matthew", "+matthew*"},
		{
			name:    "compound identifier splits into required words",
			keyword: "azn_luk_text_reg",
			want:    "+azn +luk +text +reg*",
		},
		{"underscore-separated", "jup_mat", "+jup +mat*"},
		{"reserved boolean-mode operators are stripped as separators", `jup"+mat`, "+jup +mat*"},
		{"only punctuation has no words", "___", ""},
		{"empty keyword has no words", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, buildBooleanFullTextTerm(c.keyword))
		})
	}
}

// TestGetMetadataCondLikeFallback checks the default (non-MySQL, or FULLTEXT index not
// yet confirmed) path: LIKE with the wildcard characters escaped, so a literal
// underscore in a keyword isn't read as "any single character".
func TestGetMetadataCondLikeFallback(t *testing.T) {
	require.False(t, fullTextIndexReady.Load(), "fullTextIndexReady must default to false for this test to exercise the fallback path")

	sql, args, err := builder.ToSQL(GetMetadataCond("jup_mat"))
	require.NoError(t, err)
	assert.Equal(t,
		"((`door43_metadata`.title LIKE ? ESCAPE '!')) OR `door43_metadata`.abbreviation=? OR (`door43_metadata`.subject LIKE ? ESCAPE '!') OR (LOWER(`door43_metadata`.language) = ?) OR (`door43_metadata`.language_title LIKE ? ESCAPE '!')",
		sql)
	assert.Equal(t, []any{"%jup!_mat%", "jup_mat", "%jup!_mat%", "jup_mat", "%jup!_mat%"}, args)
}

// TestGetMetadataCondFullText checks the MySQL path once EnsureFullTextIndexes has
// confirmed the index exists: title/subject/language_title go through MATCH...AGAINST
// instead of LIKE.
func TestGetMetadataCondFullText(t *testing.T) {
	origType := setting.Database.Type
	setting.Database.Type = "mysql"
	fullTextIndexReady.Store(true)
	t.Cleanup(func() {
		setting.Database.Type = origType
		fullTextIndexReady.Store(false)
	})

	sql, args, err := builder.ToSQL(GetMetadataCond("jup_mat"))
	require.NoError(t, err)
	assert.Equal(t,
		"(MATCH(`door43_metadata`.title, `door43_metadata`.subject, `door43_metadata`.language_title) AGAINST(? IN BOOLEAN MODE)) OR `door43_metadata`.abbreviation=? OR (LOWER(`door43_metadata`.language) = ?)",
		sql)
	assert.Equal(t, []any{"+jup +mat*", "jup_mat", "jup_mat"}, args)
}

// TestGetMetadataCondFullTextSkipsEmptyTerm checks that an all-punctuation keyword
// (buildBooleanFullTextTerm returns "") omits the MATCH branch entirely rather than
// sending AGAINST an empty or invalid boolean-mode string.
func TestGetMetadataCondFullTextSkipsEmptyTerm(t *testing.T) {
	origType := setting.Database.Type
	setting.Database.Type = "mysql"
	fullTextIndexReady.Store(true)
	t.Cleanup(func() {
		setting.Database.Type = origType
		fullTextIndexReady.Store(false)
	})

	sql, args, err := builder.ToSQL(GetMetadataCond("___"))
	require.NoError(t, err)
	assert.Equal(t,
		"`door43_metadata`.abbreviation=? OR (LOWER(`door43_metadata`.language) = ?)",
		sql)
	assert.Equal(t, []any{"___", "___"}, args)
}

func TestFullTextSearchAvailable(t *testing.T) {
	origType := setting.Database.Type
	t.Cleanup(func() {
		setting.Database.Type = origType
		fullTextIndexReady.Store(false)
	})

	setting.Database.Type = "sqlite3"
	fullTextIndexReady.Store(true)
	assert.False(t, fullTextSearchAvailable(), "must stay false outside MySQL even if the index flag is set")

	setting.Database.Type = "mysql"
	fullTextIndexReady.Store(false)
	assert.False(t, fullTextSearchAvailable(), "must stay false until EnsureFullTextIndexes confirms the index")

	fullTextIndexReady.Store(true)
	assert.True(t, fullTextSearchAvailable())
}
