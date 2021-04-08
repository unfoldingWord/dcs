// Copyright 2021 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dcs

import (
	"strings"
)

// Subjects are the valid subjects keyed by their resource ID
var Subjects = map[string]string{
	"obs-sn":      "OBS Study Notes",
	"obs-sq":      "OBS Study Questions",
	"obs-tn":      "OBS Translation Notes",
	"obs-tq":      "OBS Translation Questions",
	"obs":         "Open Bible Stories",
	"sn":          "Study Notes",
	"sq":          "Study Questions",
	"ta":          "Translation Academy",
	"tn":          "Translation Notes",
	"tq":          "Translation Questions",
	"tw":          "Translation Words",
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
}

// GetSubjectFromRepoName determines the subject of a repo by its repo name
func GetSubjectFromRepoName(repoName string) string {
	parts := strings.Split(repoName, "_")
	if len(parts) > 1 {
		if _, ok := Subjects[parts[1]]; ok {
			return Subjects[parts[1]]
		}
	}
	return ""
}
