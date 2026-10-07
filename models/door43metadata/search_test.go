// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"testing"

	"gitea.dev/modules/optional"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

func TestGetLowerMatchCond(t *testing.T) {
	cases := []struct {
		name         string
		values       []string
		partialMatch bool
		wantSQL      string
		wantArgs     []any
	}{
		{
			name:    "no values matches everything",
			values:  nil,
			wantSQL: "",
		},
		{
			name:     "single value is lowered on both sides",
			values:   []string{"Aligned Bible"},
			wantSQL:  "(LOWER(`door43_metadata`.subject) = ?)",
			wantArgs: []any{"aligned bible"},
		},
		{
			name:     "comma separated values are split and trimmed",
			values:   []string{"Aligned Bible, TSV Translation Notes"},
			wantSQL:  "(LOWER(`door43_metadata`.subject) = ?) OR (LOWER(`door43_metadata`.subject) = ?)",
			wantArgs: []any{"aligned bible", "tsv translation notes"},
		},
		{
			name:         "partial match wraps the lowered value",
			values:       []string{"Translation Notes"},
			partialMatch: true,
			wantSQL:      "(LOWER(`door43_metadata`.subject) LIKE ? ESCAPE '!')",
			wantArgs:     []any{"%translation notes%"},
		},
		{
			name:         "partial match escapes LIKE wildcards in the value",
			values:       []string{"tsv_notes"},
			partialMatch: true,
			wantSQL:      "(LOWER(`door43_metadata`.subject) LIKE ? ESCAPE '!')",
			wantArgs:     []any{"%tsv!_notes%"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sql, args, err := builder.ToSQL(GetLowerMatchCond("`door43_metadata`.subject", c.values, c.partialMatch))
			require.NoError(t, err)
			assert.Equal(t, c.wantSQL, sql)
			assert.Equal(t, c.wantArgs, args)
		})
	}
}

func TestSubjectFlavorCondsAreCaseInsensitive(t *testing.T) {
	// The DB collation is case-sensitive (see models/db/collation.go), so these
	// filters must lower both the column and the given value.
	cases := []struct {
		name    string
		cond    builder.Cond
		col     string
		wantArg string
	}{
		{"subject", GetSubjectCond([]string{"Aligned Bible"}, false), "`door43_metadata`.subject", "aligned bible"},
		{"flavor type", GetFlavorTypeCond([]string{"Scripture"}, false), "`door43_metadata`.flavor_type", "scripture"},
		{"flavor", GetFlavorCond([]string{"TextTranslation"}, false), "`door43_metadata`.flavor", "texttranslation"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sql, args, err := builder.ToSQL(c.cond)
			require.NoError(t, err)
			assert.Equal(t, "(LOWER("+c.col+") = ?)", sql)
			assert.Equal(t, []any{c.wantArg}, args)
		})
	}
}

// TestGetMetadataCondEscapesLikeWildcards checks that a literal "_" or "%" in a keyword
// is escaped rather than read as a LIKE wildcard, and that the escape character is
// declared explicitly (SQLite has no default one).
func TestGetMetadataCondEscapesLikeWildcards(t *testing.T) {
	sql, args, err := builder.ToSQL(GetMetadataCond("jup_mat"))
	require.NoError(t, err)
	assert.Equal(t,
		"((`door43_metadata`.title LIKE ? ESCAPE '!')) OR `door43_metadata`.abbreviation=? OR (`door43_metadata`.subject LIKE ? ESCAPE '!') OR `door43_metadata`.language=? OR (`door43_metadata`.language_title LIKE ? ESCAPE '!')",
		sql)
	assert.Equal(t, []any{"%jup!_mat%", "jup_mat", "%jup!_mat%", "jup_mat", "%jup!_mat%"}, args)
}

// TestGetLanguageCondRepoNamePattern checks the "<lang>_" repo-name convention match:
// the underscore after the code is a literal (escaped), the code itself is escaped too
// so a language value can't smuggle in a wildcard, and the escape character is declared
// (the old "\_" form relied on MySQL's default and never matched on SQLite).
func TestGetLanguageCondRepoNamePattern(t *testing.T) {
	sql, args, err := builder.ToSQL(GetLanguageCond([]string{"en"}, false))
	require.NoError(t, err)
	assert.Equal(t, "`door43_metadata`.language=? OR (`repository`.lower_name LIKE ? ESCAPE '!')", sql)
	assert.Equal(t, []any{"en", "en!_%"}, args)

	sql, args, err = builder.ToSQL(GetLanguageCond([]string{"En"}, true))
	require.NoError(t, err)
	assert.Equal(t, "(`door43_metadata`.language LIKE ? ESCAPE '!') OR (`repository`.lower_name LIKE ? ESCAPE '!')", sql)
	assert.Equal(t, []any{"%en%", "%en!_%"}, args)
}

func TestRepoMetadataIDsCond(t *testing.T) {
	// no metadata condition: matches everything, no subquery
	assert.False(t, RepoMetadataIDsCond(builder.NewCond()).IsValid())
	assert.False(t, RepoMetadataIDsCond(nil).IsValid())

	sql, args, err := builder.ToSQL(RepoMetadataIDsCond(GetSubjectCond([]string{"Bible"}, false)))
	require.NoError(t, err)
	assert.Equal(t, "`repository`.id IN (SELECT repo_id FROM door43_metadata WHERE `door43_metadata`.is_repo_metadata=? AND ((LOWER(`door43_metadata`.subject) = ?)))", sql)
	assert.Equal(t, []any{true, "bible"}, args)
}

func TestRepoIsHealthyCond(t *testing.T) {
	healthy := "`repository`.id IN (SELECT repo_id FROM door43_metadata WHERE `door43_metadata`.is_repo_metadata=? AND `door43_metadata`.healthcheck_severity IN (?,?,?))"
	cases := []struct {
		name                   string
		isHealthy, withoutWarn optional.Option[bool]
		wantSQL                string
		wantArgs               []any
	}{
		{"unset", optional.None[bool](), optional.None[bool](), "", nil},
		{"healthy", optional.Some(true), optional.None[bool](), healthy, []any{true, SeverityLevelSuccess, SeverityLevelInfo, SeverityLevelWarning}},
		// the complement, so repos without a (checked) repo entry are not healthy
		{"not healthy", optional.Some(false), optional.None[bool](), "NOT " + healthy, []any{true, SeverityLevelSuccess, SeverityLevelInfo, SeverityLevelWarning}},
		{
			"not healthy without warnings", optional.None[bool](), optional.Some(false),
			"NOT `repository`.id IN (SELECT repo_id FROM door43_metadata WHERE `door43_metadata`.is_repo_metadata=? AND `door43_metadata`.healthcheck_severity IN (?,?))",
			[]any{true, SeverityLevelSuccess, SeverityLevelInfo},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sql, args, err := builder.ToSQL(RepoIsHealthyCond(c.isHealthy, c.withoutWarn))
			require.NoError(t, err)
			assert.Equal(t, c.wantSQL, sql)
			assert.Equal(t, c.wantArgs, args)
		})
	}
}

func TestRepoOwnerCond(t *testing.T) {
	assert.False(t, RepoOwnerCond(nil, false).IsValid())

	sql, args, err := builder.ToSQL(RepoOwnerCond([]string{"unfoldingWord"}, false))
	require.NoError(t, err)
	assert.Equal(t, "`repository`.owner_id IN (SELECT id FROM `user` WHERE `user`.lower_name=?)", sql)
	assert.Equal(t, []any{"unfoldingword"}, args)
}

func TestRepoLanguageIDsSQL(t *testing.T) {
	sql, args, err := RepoLanguageIDsSQL(nil, false)
	require.NoError(t, err)
	assert.Empty(t, sql)
	assert.Nil(t, args)

	// exact: the repo entry's language, or a repo named "<lang>_..."
	sql, args, err = RepoLanguageIDsSQL([]string{"En, fr"}, false)
	require.NoError(t, err)
	assert.Equal(t, "SELECT repo_id FROM door43_metadata WHERE `door43_metadata`.is_repo_metadata=? AND (`door43_metadata`.language=? OR `door43_metadata`.language=?) UNION SELECT id FROM repository WHERE (`repository`.lower_name LIKE ? ESCAPE '!') OR (`repository`.lower_name LIKE ? ESCAPE '!')", sql)
	assert.Equal(t, []any{true, "en", "fr", "en!_%", "fr!_%"}, args)

	// partial: contained in the language code, or in the repo name followed by "_"
	sql, args, err = RepoLanguageIDsSQL([]string{"e_n"}, true)
	require.NoError(t, err)
	assert.Equal(t, "SELECT repo_id FROM door43_metadata WHERE `door43_metadata`.is_repo_metadata=? AND ((`door43_metadata`.language LIKE ? ESCAPE '!')) UNION SELECT id FROM repository WHERE (`repository`.lower_name LIKE ? ESCAPE '!')", sql)
	assert.Equal(t, []any{true, "%e!_n%", "%e!_n!_%"}, args)
}

func TestParseRepoSearchKeyword(t *testing.T) {
	fields, keyword := ParseRepoSearchKeyword("")
	assert.Empty(t, keyword)
	assert.Len(t, fields, len(RepoSearchKeywordFields))
	for _, field := range RepoSearchKeywordFields {
		assert.Empty(t, fields[field], field)
	}

	// unprefixed tokens continue the current field; "keyword" is the initial one
	fields, keyword = ParseRepoSearchKeyword(" tn , tq, lang:en, fr ,subject: Bible,keyword:obs")
	assert.Equal(t, "tn,tq,obs", keyword)
	assert.Equal(t, []string{"tn", "tq", "obs"}, fields["keyword"])
	assert.Equal(t, []string{"en", "fr"}, fields["lang"])
	assert.Equal(t, []string{"Bible"}, fields["subject"])
	assert.Empty(t, fields["flavor"])

	// a field prefix must match a whole field name
	fields, keyword = ParseRepoSearchKeyword("flavor_type:scripture, without_topic:x, unknown:y")
	assert.Equal(t, []string{"scripture"}, fields["flavor_type"])
	assert.Empty(t, fields["flavor"])
	assert.Equal(t, []string{"x", "unknown:y"}, fields["without_topic"], "an unknown prefix stays a value of the current field")
	assert.Empty(t, fields["topic"])
	assert.Empty(t, keyword)
}
