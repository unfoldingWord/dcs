// Copyright 2026 The DCS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package languagestats

// dcsIncludedFileTypes are the Bible-translation content formats that DCS
// always counts in repository language stats, even though go-enry classifies
// them as Data/Prose (TSV, CSV, Text, reStructuredText) or does not recognize
// them at all (USFM). Without this, only Programming and Markup languages are
// counted and these formats are dropped from the "File Types" bar.
var dcsIncludedFileTypes = map[string]bool{
	"USFM":             true,
	"Markdown":         true,
	"CSV":              true,
	"TSV":              true,
	"Text":             true,
	"reStructuredText": true,
}

// DCSIsIncludedFileType reports whether the given detected language is a DCS
// content file type that should always be included in language stats.
func DCSIsIncludedFileType(language string) bool {
	return dcsIncludedFileTypes[language]
}
