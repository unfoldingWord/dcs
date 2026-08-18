// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// GetCsvCellDiff returns the diff of two strings
func GetCsvCellDiff(old, cur string) template.HTML {
	dmp := diffmatchpatch.New()

	diffs := dmp.DiffMain(old, cur, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	if len(diffs) == 0 {
		return template.HTML(fmt.Sprintf("<span class=\"removed-code\">%s</span><span class=\"added-code\">%s</span>", old, cur))
	}

	return template.HTML(writeDiffHTML(diffs))
}

func writeDiffHTML(diffs []diffmatchpatch.Diff) string {
	var removedCode, addedCode strings.Builder
	removed := false
	added := false

	// write the diff
	for _, chunk := range diffs {
		txt := html.EscapeString(chunk.Text)
		txt = strings.ReplaceAll(txt, "\n", "↩\n")
		switch chunk.Type {
		case diffmatchpatch.DiffInsert:
			addedCode.WriteString(`<span class="added-code">`)
			addedCode.WriteString(txt)
			addedCode.WriteString(`</span>`)
			added = true
		case diffmatchpatch.DiffDelete:
			removedCode.WriteString(`<span class="removed-code">`)
			removedCode.WriteString(txt)
			removedCode.WriteString(`</span>`)
			removed = true
		case diffmatchpatch.DiffEqual:
			addedCode.WriteString(txt)
			removedCode.WriteString(txt)
		}
	}

	if added && removed {
		return fmt.Sprintf(`<div class="del-code">%s</div><div class="add-code">%s</div>`, removedCode.String(), addedCode.String())
	} else if added {
		return fmt.Sprintf(`<div class="add-code">%s</div>`, addedCode.String())
	} else if removed {
		return fmt.Sprintf(`<div class="del-code">%s</div>`, removedCode.String())
	}
	return fmt.Sprintf(`<div class="same-code">%s</div>`, addedCode.String())
}

// GetMetadataTypeFromRepoName determines the metadata type of a repo by its repo name format
func GetMetadataTypeFromRepoName(repoName string) string {
	parts := strings.Split(strings.ToLower(repoName), "_")
	if len(parts) == 2 && IsValidLanguage(parts[0]) && IsValidResource(parts[1]) {
		return "rc"
	}
	if len(parts) == 4 && IsValidLanguage(parts[0]) && IsValidBook(parts[2]) && parts[3] == "book" {
		return "tc"
	}
	if len(parts) == 4 && IsValidLanguage(parts[0]) && IsValidBook(parts[1]) && parts[2] == "text" {
		return "ts"
	}
	parts = strings.Split(strings.ToLower(repoName), "-")
	if len(parts) == 3 && IsValidLanguage(parts[0]) && (parts[1] == "textstories" || parts[1] == "texttranslation") {
		return "sb"
	}
	return ""
}

// GetMetadataVersionFromRepoName returns the default version for each metadata type based on given metadata type
func GetDefaultMetadataVersionForType(metadataType string) string {
	if metadataType == "rc" {
		return "0.2"
	}
	if metadataType == "sb" {
		return "1.0.0"
	}
	if metadataType == "tc" {
		return "8"
	}
	if metadataType == "ts" {
		return "7"
	}
	return ""
}
