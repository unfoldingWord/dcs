// Copyright 2026 The DCS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package languagestats

import (
	"path"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

// DCSResolveLanguage returns the file-type label to count in the repository
// language stats. DCS aims to represent every file type, not only the
// Programming/Markup languages go-enry counts by default. When go-enry cannot
// classify a file (binary documents such as PDF/DOCX/XLSX/PPTX, audio/video
// such as MP3/MP4/MOV/MKV, etc.) we fall back to a label derived from the file
// extension, e.g. ".pdf" -> "PDF", ".mov" -> "MOV". This is generic, so any
// extension is represented without maintaining a list.
//
// Returns "" only when there is nothing to attribute (no go-enry language and
// no file extension), in which case the caller skips the file.
func DCSResolveLanguage(filename, enryLanguage string) string {
	// enry.OtherLanguage is the empty string; keep any real go-enry result.
	if enryLanguage != enry.OtherLanguage {
		return enryLanguage
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(filename)), ".")
	if ext == "" {
		return ""
	}
	return strings.ToUpper(ext)
}
