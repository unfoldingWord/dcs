// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"testing"

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
			wantSQL:      "(LOWER(`door43_metadata`.subject) LIKE ?)",
			wantArgs:     []any{"%translation notes%"},
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
