// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"strings"

	rc2sb "github.com/unfoldingWord/go-rc2sb"
)

// SubjectToResourceMap maps subjects to their valid resource IDs
var SubjectToResourceMap = map[string][]string{
	"Aligned Bible":                   {"glt", "gst", "ult", "ust"},
	"OBS Study Notes":                 {"obs-sn"},
	"OBS Study Questions":             {"obs-sq"},
	"OBS Theological Formation":       {"obs-tf"},
	"OBS Translation Notes":           {"obs-tn"},
	"OBS Translation Questions":       {"obs-tq"},
	"Open Bible Stories":              {"obs"},
	"TSV OBS Translation Words Links": {"obs-twl"},
	"Study Notes":                     {"sn"},
	"Study Questions":                 {"sq"},
	"Translation Academy":             {"ta"},
	"Training Library":                {"tl"},
	"TSV Translation Notes":           {"tn"},
	"TSV Translation Questions":       {"tq"},
	"Translation Words":               {"tw"},
	"TSV Translation Words Links":     {"twl"},
	"TSV Study Notes":                 {"sn"},
	"TSV Study Questions":             {"sq"},
	"TSV OBS Study Notes":             {"obs-sn"},
	"TSV OBS Study Questions":         {"obs-sq"},
	"TSV OBS Translation Notes":       {"obs-tn"},
	"TSV OBS Translation Questions":   {"obs-tq"},
	"Greek New Testament":             {"ugnt"},
	"Hebrew Old Testament":            {"uhb"},
}

// ResourceToSubjectMap is the inverse of SubjectToResourceMap. A resource ID listed
// under two subjects maps to the TSV one, as map order would otherwise pick at random.
var ResourceToSubjectMap = func() map[string]string {
	m := make(map[string]string)
	for subject, resources := range SubjectToResourceMap {
		for _, resource := range resources {
			if _, taken := m[resource]; !taken || strings.HasPrefix(subject, "TSV ") {
				m[resource] = subject
			}
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

// SBFlavor is a Scripture Burrito flavor type and flavor
type SBFlavor struct {
	FlavorType string
	Flavor     string
}

// ConvertedSBFlavors are the flavors of the burritos go-rc2sb converts an RC of each subject
// to, so that a catalog entry agrees with its /sb/ download. TestConvertedSBFlavors keeps
// it in step with the converter.
var ConvertedSBFlavors = map[string]SBFlavor{
	"Aligned Bible":                   {"scripture", "textTranslation"},
	"Bible":                           {"scripture", "textTranslation"},
	"Hebrew Old Testament":            {"scripture", "textTranslation"},
	"Greek New Testament":             {"scripture", "textTranslation"},
	"Open Bible Stories":              {"gloss", "textStories"},
	"Translation Academy":             {"peripheral", "x-peripheralArticles"},
	"Translation Words":               {"parascriptural", "x-bcvarticles"},
	"TSV Translation Words Links":     {"parascriptural", "x-bcvarticles"},
	"TSV OBS Translation Words Links": {"parascriptural", "x-bcvarticles"},
	"Translation Notes":               {"parascriptural", "x-bcvnotes"},
	"Study Notes":                     {"parascriptural", "x-bcvnotes"},
	"Translation Questions":           {"parascriptural", "x-bcvquestions"},
	"Study Questions":                 {"parascriptural", "x-bcvquestions"},
	"TSV Translation Notes":           {"parascriptural", "x-bcvnotes"},
	"TSV Study Notes":                 {"parascriptural", "x-bcvnotes"},
	"TSV Translation Questions":       {"parascriptural", "x-bcvquestions"},
	"TSV Study Questions":             {"parascriptural", "x-bcvquestions"},
	"TSV OBS Translation Notes":       {"peripheral", "x-obsnotes"},
	"TSV OBS Study Notes":             {"peripheral", "x-obsnotes"},
	"OBS Translation Notes":           {"peripheral", "x-obsnotes"},
	"OBS Study Notes":                 {"peripheral", "x-obsnotes"},
	"TSV OBS Translation Questions":   {"peripheral", "x-obsquestions"},
	"TSV OBS Study Questions":         {"peripheral", "x-obsquestions"},
	"OBS Translation Questions":       {"peripheral", "x-obsquestions"},
	"OBS Study Questions":             {"peripheral", "x-obsquestions"},
}

// tsSBSubjects are the ts subjects go-ts2rc converts to an RC that go-rc2sb converts
var tsSBSubjects = map[string]bool{
	"Bible":                     true,
	"Open Bible Stories":        true,
	"OBS Translation Notes":     true,
	"OBS Translation Questions": true,
	"Translation Notes":         true,
	"Translation Questions":     true,
	"Translation Words":         true,
	"Translation Academy":       true,
}

// SBConvertible reports whether a repo of the metadata type and subject has a Scripture
// Burrito download: an sb repo is one already, a tc repo converts by way of go-tc2rc, a ts
// repo by way of go-ts2rc, and an rc repo when go-rc2sb converts its subject.
func SBConvertible(metadataType, subject string) bool {
	switch metadataType {
	case "sb", "tc":
		return true
	case "ts":
		return tsSBSubjects[subject]
	case "rc":
		return rc2sb.Supports(subject)
	}
	return false
}
