// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeOBSStory(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		hasTitle    bool
		hasFrame    bool
		hasBibleRef bool
	}{
		{
			name: "valid English story",
			content: "# 1. The Creation\n\n![OBS Image](https://cdn.door43.org/obs/jpg/360px/obs-en-01-01.jpg)\n\n" +
				"This is how the beginning of everything happened.\n\n_A Bible story from: Genesis 1-2_\n",
			hasTitle: true, hasFrame: true, hasBibleRef: true,
		},
		{
			name: "valid non-English story without numbered title",
			content: "# सृष्टि की कहानी\n\n![OBS Image](https://cdn.door43.org/obs/jpg/360px/obs-hi-01-01.jpg)\n\n" +
				"परमेश्वर ने छ: दिनों में सब कुछ बनाया।\n\n_बाइबल की एक कहानी: उत्पत्ति 1-2_\n",
			hasTitle: true, hasFrame: true, hasBibleRef: true,
		},
		{
			name:     "trailing blank lines after Bible reference are allowed",
			content:  "# Titre\n\n![image](img.jpg)\n\n_Une histoire biblique tirée de: Genèse 1-2_\n\n\n",
			hasTitle: true, hasFrame: true, hasBibleRef: true,
		},
		{
			name:     "Bible reference line may have trailing whitespace",
			content:  "# Title\n\n![image](img.jpg)\n\n_A Bible story from: Genesis 1-2_  \n",
			hasTitle: true, hasFrame: true, hasBibleRef: true,
		},
		{
			name:     "UTF-8 BOM before title is ignored",
			content:  "\ufeff# Title\n\n![image](img.jpg)\n\n_Reference_\n",
			hasTitle: true, hasFrame: true, hasBibleRef: true,
		},
		{
			name:     "title not on first line",
			content:  "\n# Title\n\n![image](img.jpg)\n\n_Reference_\n",
			hasTitle: false, hasFrame: true, hasBibleRef: true,
		},
		{
			name:     "title without following blank line",
			content:  "# Title\n![image](img.jpg)\n\n_Reference_\n",
			hasTitle: false, hasFrame: true, hasBibleRef: true,
		},
		{
			name:     "level-2 heading is not a title",
			content:  "## Title\n\n![image](img.jpg)\n\n_Reference_\n",
			hasTitle: false, hasFrame: true, hasBibleRef: true,
		},
		{
			name:     "Bible reference not preceded by blank line",
			content:  "# Title\n\n![image](img.jpg)\nSome text.\n_Reference_\n",
			hasTitle: true, hasFrame: true, hasBibleRef: false,
		},
		{
			name:     "text after Bible reference",
			content:  "# Title\n\n![image](img.jpg)\n\n_Reference_\n\nTrailing text.\n",
			hasTitle: true, hasFrame: true, hasBibleRef: false,
		},
		{
			name:     "last line not italicized",
			content:  "# Title\n\n![image](img.jpg)\n\nA Bible story from: Genesis 1-2\n",
			hasTitle: true, hasFrame: true, hasBibleRef: false,
		},
		{
			name:     "no frame image",
			content:  "# Title\n\nSome text.\n\n_Reference_\n",
			hasTitle: true, hasFrame: false, hasBibleRef: true,
		},
		{
			name:    "empty file",
			content: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasTitle, hasFrame, hasBibleRef := analyzeOBSStory(strings.NewReader(tt.content))
			assert.Equal(t, tt.hasTitle, hasTitle, "hasTitle")
			assert.Equal(t, tt.hasFrame, hasFrame, "hasFrame")
			assert.Equal(t, tt.hasBibleRef, hasBibleRef, "hasBibleRef")
		})
	}
}
