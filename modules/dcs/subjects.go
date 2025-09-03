// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"strings"
)

// SubjectToResourceMap maps subjects to their valid resource IDs
var SubjectToResourceMap = map[string][]string{
	"Aligned Bible":                   {"glt", "gst", "ult", "ust"},
	"OBS Study Notes":                 {"obs-sn2"},
	"OBS Study Questions":             {"obs-sq"},
	"OBS Translation Notes":           {"obs-tn"},
	"OBS Translation Questions":       {"obs-tq"},
	"Open Bible Stories":              {"obs"},
	"TSV OBS Translation Words Links": {"obs-twl"},
	"Study Notes":                     {"sn"},
	"Study Questions":                 {"sq"},
	"Translation Academy":             {"ta"},
	"Training Library":                {"tl"},
	"TSV Translation Notes":           {"tn", "tn-tsv"},
	"TSV Translation Questions":       {"tq", "tq-tsv"},
	"Translation Words":               {"tw"},
	"TSV Translation Words Links":     {"twl"},
	"TSV Study Notes":                 {"sn-tsv"},
	"TSV Study Questions":             {"sq-tsv"},
	"TSV OBS Study Notes":             {"obs-sn"},
	"TSV OBS Study Questions":         {"obs-sq"},
	"TSV OBS Translation Notes":       {"obs-tn"},
	"TSV OBS Translation Questions":   {"obs-tq"},
	"Greek New Testament":             {"ugnt"},
	"Hebrew Old Testament":            {"uhb"},
}

// ResourceToSubjectMap is the inverse of SubjectToResourceMap
var ResourceToSubjectMap = func() map[string]string {
	m := make(map[string]string)
	for subject, resources := range SubjectToResourceMap {
		for _, resource := range resources {
			m[resource] = subject
		}
	}
	return m
}()

// GetSubjectFromRepoName determines the subject of a repo by its repo name
func GetSubjectFromRepoName(repoName string) string {
	parts := strings.Split(strings.ToLower(repoName), "_")
	if len(parts) == 2 && IsValidResource(parts[1]) && IsValidLanguage(parts[0]) {
		return ResourceToSubjectMap[parts[1]]
	}
	if len(parts) == 4 && IsValidLanguage(parts[0]) && IsValidBook(parts[2]) && parts[3] == "book" {
		return "Aligned Bible"
	}
	if len(parts) == 4 && IsValidLanguage(parts[0]) && IsValidBook(parts[1]) && parts[2] == "text" {
		if parts[1] == "obs" {
			return "Open Bible Stories"
		}
		return "Bible"
	}
	parts = strings.Split(repoName, "-")
	if len(parts) == 3 && IsValidLanguage(parts[0]) {
		if parts[1] == "textstories" {
			return "Open Bible Stories"
		} else if parts[2] == "texttranslation" {
			return "Bible"
		}
	}
	return ""
}

// IsValidResource returns true if it is a valid resource
func IsValidResource(str string) bool {
	_, ok := ResourceToSubjectMap[str]
	return ok
}
