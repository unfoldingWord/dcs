// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"strings"
)

// Subjects are the valid subjects keyed by their resource ID
var Subjects = map[string]string{
	"glt":         "Aligned Bible",
	"gst":         "Aligned Bible",
	"obs-sn":      "OBS Study Notes",
	"obs-sq":      "OBS Study Questions",
	"obs-tn":      "OBS Translation Notes",
	"obs-tq":      "OBS Translation Questions",
	"obs":         "Open Bible Stories",
	"obs-twl":     "TSV OBS Translation Words Links",
	"sn":          "Study Notes",
	"sq":          "Study Questions",
	"ta":          "Translation Academy",
	"tl":          "Training Library",
	"tn":          "TSV Translation Notes",
	"tq":          "TSV Translation Questions",
	"tw":          "Translation Words",
	"twl":         "TSV Translation Word Links",
	"sn-tsv":      "TSV Study Notes",
	"sq-tsv":      "TSV Study Questions",
	"tn-tsv":      "TSV Translation Notes",
	"tq-tsv":      "TSV Translation Questions",
	"twl-tsv":     "TSV Translation Words Links",
	"obs-sn-tsv":  "TSV OBS Study Notes",
	"obs-sq-tsv":  "TSV OBS Study Questions",
	"obs-tn-tsv":  "TSV OBS Translation Notes",
	"obs-tq-tsv":  "TSV OBS Translation Questions",
	"obs-twl-tsv": "TSV OBS Translation Words Links",
	"ult":         "Aligned Bible",
	"ust":         "Aligned Bible",
}

// GetSubjectFromRepoName determines the subject of a repo by its repo name
func GetSubjectFromRepoName(repoName string) string {
	parts := strings.Split(repoName, "_")
	if len(parts) == 2 && IsValidSubject(parts[1]) && IsValidLanguage(parts[0]) {
		return Subjects[parts[1]]
	}
	return ""
}

// IsValidSubject returns true if it is a valid subject
func IsValidSubject(subject string) bool {
	_, ok := Subjects[subject]
	return ok
}
