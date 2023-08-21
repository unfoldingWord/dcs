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
func GetCsvCellDiff(old, new string) template.HTML {
	dmp := diffmatchpatch.New()

	diffs := dmp.DiffMain(old, new, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	if len(diffs) == 0 {
		return template.HTML(fmt.Sprintf("<span class=\"removed-code\">%s</span><span class=\"added-code\">%s</span>", old, new))
	}

	return template.HTML(writeDiffHTML(diffs))
}

func writeDiffHTML(diffs []diffmatchpatch.Diff) string {
	removedCode := ""
	removed := false
	addedCode := ""
	added := false

	// write the diff
	for _, chunk := range diffs {
		txt := html.EscapeString(chunk.Text)
		txt = strings.ReplaceAll(txt, "\n", "↩\n")
		switch chunk.Type {
		case diffmatchpatch.DiffInsert:
			addedCode += `<span class="added-code">`
			addedCode += txt
			addedCode += `</span>`
			added = true
		case diffmatchpatch.DiffDelete:
			removedCode += `<span class="removed-code">`
			removedCode += txt
			removedCode += `</span>`
			removed = true
		case diffmatchpatch.DiffEqual:
			addedCode += txt
			removedCode += txt
		}
	}

	if added && removed {
		return fmt.Sprintf(`<div class="del-code">%s</div><div class="add-code">%s</div>`, removedCode, addedCode)
	} else if added {
		return fmt.Sprintf(`<div class="add-code">%s</div>`, addedCode)
	} else if removed {
		return fmt.Sprintf(`<div class="del-code">%s</div>`, removedCode)
	}
	return fmt.Sprintf(`<div class="same-code">%s</div>`, addedCode)
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
