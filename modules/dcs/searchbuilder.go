// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"cmp"
	"slices"
	"strings"

	"gitea.dev/modules/log"
)

// SearchBuilderField is one input of the search builder shown above the repo and catalog
// lists (templates/dcs/search_builder.tmpl). Name is the "field:" prefix the q parameter
// parsers accept; Options, when set, feed the input's datalist.
type SearchBuilderField struct {
	Name    string
	Options []string
}

// Display order. The catalog-only fields come last; the repo list parser ignores them.
var (
	searchBuilderRepoFields    = []string{"keyword", "lang", "topic", "without_topic", "subject", "owner", "repo", "abbreviation", "book", "flavor_type", "flavor", "metadata_type", "metadata_version", "content_format", "healthcheck"}
	searchBuilderCatalogFields = []string{"tag", "stage", "checking_level", "has", "include_history", "is_healthy", "is_healthy_without_warnings"}
)

var searchBuilderOptions = map[string][]string{
	"flavor_type":                 {"gloss", "parascriptural", "peripheral", "scripture"},
	"flavor":                      {"audioTranslation", "embossedBrailleScripture", "signLanguageVideoTranslation", "textStories", "textTranslation", "typesetScripture", "wordAlignment"},
	"metadata_type":               {"rc", "sb", "tc", "ts"},
	"content_format":              {"markdown", "tsv7", "tsv9", "usfm"},
	"healthcheck":                 {"error", "warning", "info", "success"},
	"stage":                       {"prod", "preprod", "latest", "other"},
	"checking_level":              {"1", "2", "3"},
	"has":                         {"attachment", "audio", "video", "pdf", "stream", "other"},
	"include_history":             {"true"},
	"is_healthy":                  {"true", "false"},
	"is_healthy_without_warnings": {"true", "false"},
}

// SearchBuilderFields returns the search builder inputs for the repo lists or, with
// catalog set, the catalog page.
func SearchBuilderFields(catalog bool) []SearchBuilderField {
	names := searchBuilderRepoFields
	if catalog {
		names = slices.Concat(searchBuilderRepoFields, searchBuilderCatalogFields)
	}
	fields := make([]SearchBuilderField, 0, len(names))
	for _, name := range names {
		var options []string
		switch name {
		case "subject":
			options = searchBuilderSubjects()
		case "abbreviation":
			options = searchBuilderAbbreviations()
		case "book":
			options = searchBuilderBooks()
		default:
			options = searchBuilderOptions[name]
		}
		fields = append(fields, SearchBuilderField{Name: name, Options: options})
	}
	return fields
}

// searchBuilderSubjects is the subject enum of the RC 0.2 schema, the one list every
// metadata type is normalised to.
func searchBuilderSubjects() []string {
	schema, err := GetRC02Schema(false)
	if err != nil {
		log.Error("SearchBuilderFields: load RC02 schema: %v", err)
		return nil
	}
	dublinCore := schema.Properties["dublin_core"]
	if dublinCore == nil || dublinCore.Properties["subject"] == nil {
		return nil
	}
	subjects := make([]string, 0, len(dublinCore.Properties["subject"].Enum))
	for _, v := range dublinCore.Properties["subject"].Enum {
		if s, ok := v.(string); ok {
			subjects = append(subjects, s)
		}
	}
	slices.Sort(subjects)
	return subjects
}

func searchBuilderAbbreviations() []string {
	var ids []string
	for _, resourceIDs := range SubjectToResourceMap {
		ids = append(ids, resourceIDs...)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

func searchBuilderBooks() []string {
	books := make([]string, 0, len(BookNames))
	for book := range BookNames {
		books = append(books, book)
	}
	slices.SortFunc(books, func(a, b string) int {
		return cmp.Or(cmp.Compare(GetBookSort(a), GetBookSort(b)), strings.Compare(a, b)) // frt/bak/obs all sort as 0
	})
	return books
}
