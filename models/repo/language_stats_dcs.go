// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

// dcsLanguageColors maps notable DCS file types to comfortable, distinct colors
// for the repository "File Types" bar. The formats DCS primarily uses (USFM,
// TSV, Markdown, reStructuredText, Text) get prominent, brand-aligned colors;
// common document and audio/video formats get comfortable colors too. Any file
// type not listed here is assigned a stable color from dcsColorPalette, so every
// type stays visually distinct.
var dcsLanguageColors = map[string]string{
	// Primary DCS content formats (highest priority)
	"USFM":             "#31ADE3", // Inspire blue
	"TSV":              "#70C9CC", // Cultivate teal
	"Markdown":         "#014263", // Ocean blue
	"reStructuredText": "#E59D33", // Kindle amber
	"Text":             "#8E7CC3", // comfortable violet
	"CSV":              "#6AA84F", // comfortable green

	// Documents
	"PDF":  "#B23A48", // crimson, kept distinct from go-enry's HTML orange-red (#e34c26)
	"DOC":  "#2B7BB9",
	"DOCX": "#2B7BB9",
	"ODT":  "#3D85C6",
	"XLS":  "#38866A",
	"XLSX": "#38866A",
	"ODS":  "#57A773",
	"PPT":  "#D9803B",
	"PPTX": "#D9803B",
	"ODP":  "#E0A458",

	// Audio
	"MP3":  "#C27BA0",
	"M4A":  "#B07CC6",
	"WAV":  "#A569BD",
	"FLAC": "#9B72CF",
	"OGG":  "#B57BBA",

	// Video
	"MP4":  "#674EA7",
	"M4V":  "#8160C8",
	"MOV":  "#7E57C2",
	"MKV":  "#5E60CE",
	"AVI":  "#6930C3",
	"WMV":  "#5390D9",
	"WEBM": "#7C6FCB",
}

// dcsColorPalette is a set of comfortable, visually distinct colors used to give
// any file type not in dcsLanguageColors a stable color.
var dcsColorPalette = []string{
	"#4E79A7", "#F28E2B", "#59A14F", "#E15759", "#76B7B2",
	"#EDC948", "#B07AA1", "#FF9DA7", "#9C755F", "#BAB0AC",
	"#86BCB6", "#D37295", "#FABFD2", "#B6992D", "#8CD17D",
	"#499894", "#A0CBE8", "#F1CE63", "#D4A6C8", "#79706E",
}

// DCSLanguageColor returns the bar color for a file-type label. Curated colors
// win; otherwise any real go-enry color (passed in) is kept; for types go-enry
// does not know (it reports those as the default grey "#cccccc") a stable,
// comfortable color is chosen from dcsColorPalette so every file type is distinct.
func DCSLanguageColor(language, enryColor string) string {
	if color, ok := dcsLanguageColors[language]; ok {
		return color
	}
	if enryColor != "" && enryColor != "#cccccc" {
		return enryColor
	}
	hash := 0
	for i := 0; i < len(language); i++ {
		hash = int(language[i]) + ((hash << 5) - hash)
	}
	if hash < 0 {
		hash = -hash
	}
	return dcsColorPalette[hash%len(dcsColorPalette)]
}
