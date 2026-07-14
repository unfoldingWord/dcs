// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package analyze

import (
	"path"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

/*** DCS Customizations ***/
// dcsFileTypeByExt maps file extensions for Bible-translation content formats
// that go-enry/linguist does not recognize to a stable language name shown in
// the repository "File Types" bar. (TSV, CSV, Markdown, Text and
// reStructuredText are already recognized by go-enry.)
var dcsFileTypeByExt = map[string]string{
	".usfm": "USFM",
	".sfm":  "USFM",
}

/*** END DCS Customizations ***/

// GetCodeLanguage detects code language based on file name and content
// It can be slow when the content is used for detection
func GetCodeLanguage(filename string, content []byte) string {
	/*** DCS Customizations ***/
	if language, ok := dcsFileTypeByExt[strings.ToLower(path.Ext(filename))]; ok {
		return language
	}
	/*** END DCS Customizations ***/

	if language, ok := enry.GetLanguageByExtension(filename); ok {
		return language
	}

	if language, ok := enry.GetLanguageByFilename(filename); ok {
		return language
	}

	if len(content) == 0 {
		return enry.OtherLanguage
	}

	return enry.GetLanguage(path.Base(filename), content)
}
