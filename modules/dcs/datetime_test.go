// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHumanDate(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
		dateOnly bool
	}{
		{"2026-07-13", time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), true},
		{"2026/07/13", time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), true},
		{"07/13/2026", time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), true},
		{"Jul 13, 2026", time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), true},
		{"July 13, 2026", time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), true},
		{"13 Jul 2026", time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), true},
		{"2026-07-13 15:04:05", time.Date(2026, 7, 13, 15, 4, 5, 0, time.UTC), false},
		{"2026-07-13 15:04", time.Date(2026, 7, 13, 15, 4, 0, 0, time.UTC), false},
		{"2026-07-13T15:04:05Z", time.Date(2026, 7, 13, 15, 4, 5, 0, time.UTC), false},
		{" 2026-07-13 ", time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), true},
		{"1752418800", time.Unix(1752418800, 0).UTC(), false},
	}
	for _, test := range tests {
		parsed, dateOnly, err := ParseHumanDate(test.input)
		require.NoError(t, err, "input: %q", test.input)
		assert.True(t, test.expected.Equal(parsed), "input: %q, got: %v", test.input, parsed)
		assert.Equal(t, test.dateOnly, dateOnly, "input: %q", test.input)
	}

	for _, invalid := range []string{"", "not a date", "13/07/2026 25:00"} {
		_, _, err := ParseHumanDate(invalid)
		assert.Error(t, err, "input: %q", invalid)
	}
}
