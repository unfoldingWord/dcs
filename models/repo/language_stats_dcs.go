// Copyright 2026 The DCS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

// dcsLanguageColors maps DCS content file types to unfoldingWord brand colors
// for the repository "File Types" bar. go-enry leaves USFM and Text without a
// color (grey), and gives CSV and TSV the same green, which makes them
// indistinguishable. These overrides keep the bar branded and legible.
var dcsLanguageColors = map[string]string{
	"USFM":             "#31ADE3", // Inspire
	"Markdown":         "#014263", // Ocean
	"TSV":              "#70C9CC", // Cultivate
	"CSV":              "#E59D33", // Kindle
	"Text":             "#231F20", // Tech
	"reStructuredText": "#7FCBEC", // Inspire (light)
}

// DCSLanguageColor returns the unfoldingWord brand color for a DCS content file
// type, or an empty string if the language has no DCS-specific color.
func DCSLanguageColor(language string) string {
	return dcsLanguageColors[language]
}
