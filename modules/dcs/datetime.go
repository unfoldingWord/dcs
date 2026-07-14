// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// humanDateTimeLayouts are the accepted layouts that include a time component,
// tried in order. All are parsed in UTC unless the string carries its own zone.
var humanDateTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
}

// humanDateLayouts are the accepted date-only layouts, tried in order.
var humanDateLayouts = []string{
	"2006-01-02",
	"2006/01/02",
	"01/02/2006",
	"Jan 2, 2006",
	"January 2, 2006",
	"2 Jan 2006",
	"2 January 2006",
}

// ParseHumanDate parses a human-entered date or date-time string in UTC. A
// string of all digits is treated as a Unix timestamp in seconds. dateOnly
// reports whether the matched layout has no time component (e.g. "2026-07-13"),
// so callers can treat an end date as inclusive of the whole day.
func ParseHumanDate(s string) (t time.Time, dateOnly bool, err error) {
	s = strings.TrimSpace(s)
	if secs, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(secs, 0).UTC(), false, nil
	}
	for _, layout := range humanDateTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, false, nil
		}
	}
	for _, layout := range humanDateLayouts {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, true, nil
		}
	}
	return time.Time{}, false, fmt.Errorf("unrecognized date format: %q", s)
}
