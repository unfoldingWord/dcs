// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"fmt"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeOBSStory(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		hasTitle    bool
		frames      int
		hasBibleRef bool
	}{
		{
			name: "valid English story",
			content: "# 1. The Creation\n\n![OBS Image](https://cdn.door43.org/obs/jpg/360px/obs-en-01-01.jpg)\n\n" +
				"This is how the beginning of everything happened.\n\n_A Bible story from: Genesis 1-2_\n",
			hasTitle: true, frames: 1, hasBibleRef: true,
		},
		{
			name: "valid non-English story without numbered title",
			content: "# सृष्टि की कहानी\n\n![OBS Image](https://cdn.door43.org/obs/jpg/360px/obs-hi-01-01.jpg)\n\n" +
				"परमेश्वर ने छ: दिनों में सब कुछ बनाया।\n\n_बाइबल की एक कहानी: उत्पत्ति 1-2_\n",
			hasTitle: true, frames: 1, hasBibleRef: true,
		},
		{
			name:     "trailing blank lines after Bible reference are allowed",
			content:  "# Titre\n\n![image](img.jpg)\n\n_Une histoire biblique tirée de: Genèse 1-2_\n\n\n",
			hasTitle: true, frames: 1, hasBibleRef: true,
		},
		{
			name:     "Bible reference line may have trailing whitespace",
			content:  "# Title\n\n![image](img.jpg)\n\n_A Bible story from: Genesis 1-2_  \n",
			hasTitle: true, frames: 1, hasBibleRef: true,
		},
		{
			name:     "UTF-8 BOM before title is ignored",
			content:  "\ufeff# Title\n\n![image](img.jpg)\n\n_Reference_\n",
			hasTitle: true, frames: 1, hasBibleRef: true,
		},
		{
			name:     "title not on first line",
			content:  "\n# Title\n\n![image](img.jpg)\n\n_Reference_\n",
			hasTitle: false, frames: 1, hasBibleRef: true,
		},
		{
			name:     "title without following blank line",
			content:  "# Title\n![image](img.jpg)\n\n_Reference_\n",
			hasTitle: false, frames: 1, hasBibleRef: true,
		},
		{
			name:     "level-2 heading is not a title",
			content:  "## Title\n\n![image](img.jpg)\n\n_Reference_\n",
			hasTitle: false, frames: 1, hasBibleRef: true,
		},
		{
			name:     "Bible reference not preceded by blank line",
			content:  "# Title\n\n![image](img.jpg)\nSome text.\n_Reference_\n",
			hasTitle: true, frames: 1, hasBibleRef: false,
		},
		{
			name:     "text after Bible reference",
			content:  "# Title\n\n![image](img.jpg)\n\n_Reference_\n\nTrailing text.\n",
			hasTitle: true, frames: 1, hasBibleRef: false,
		},
		{
			name:     "last line not italicized",
			content:  "# Title\n\n![image](img.jpg)\n\nA Bible story from: Genesis 1-2\n",
			hasTitle: true, frames: 1, hasBibleRef: false,
		},
		{
			name:     "no frame image",
			content:  "# Title\n\nSome text.\n\n_Reference_\n",
			hasTitle: true, frames: 0, hasBibleRef: true,
		},
		{
			name:     "frames are counted per image line",
			content:  "# Title\n\n![a](1.jpg)\n\nOne.\n\n![b](2.jpg)\n\nTwo.\n\n![c](3.jpg)\n\nThree.\n\n_Reference_\n",
			hasTitle: true, frames: 3, hasBibleRef: true,
		},
		{
			name:    "empty file",
			content: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			story := analyzeOBSStory(strings.NewReader(tt.content))
			assert.Equal(t, tt.hasTitle, story.hasTitle, "hasTitle")
			assert.Equal(t, tt.frames, story.frames, "frames")
			assert.Equal(t, tt.hasBibleRef, story.hasBibleRef, "hasBibleRef")
		})
	}
}

func TestOBSStoryIssues(t *testing.T) {
	var allButFirst []string
	for i := 2; i <= 50; i++ {
		allButFirst = append(allButFirst, fmt.Sprintf("%02d", i))
	}
	complete := obsStory{hasTitle: true, frames: minOBSFrames, hasBibleRef: true}
	codes := func(issues []*repo_model.Door43HealthcheckIssue) map[repo_model.IssueCode]repo_model.SeverityLevel {
		m := map[repo_model.IssueCode]repo_model.SeverityLevel{}
		for _, issue := range issues {
			m[issue.IssueCode] = issue.SeverityLevel
		}
		return m
	}

	t.Run("lone 01 with at most one frame is a placeholder", func(t *testing.T) {
		assert.Empty(t, obsStoryIssues(allButFirst, []obsStory{{num: "01", hasTitle: true, frames: 1}}))
		assert.Empty(t, obsStoryIssues(allButFirst, []obsStory{{num: "01"}}))
	})

	t.Run("lone 01 with more frames is checked", func(t *testing.T) {
		got := codes(obsStoryIssues(allButFirst, []obsStory{{num: "01", hasTitle: true, frames: 2, hasBibleRef: true}}))
		assert.Equal(t, map[repo_model.IssueCode]repo_model.SeverityLevel{
			repo_model.IssueCodeOBSStoryMissing:    repo_model.SeverityLevelError,
			repo_model.IssueCodeOBSWrongFrameCount: repo_model.SeverityLevelError,
		}, got)
	})

	t.Run("lone story other than 01 is checked", func(t *testing.T) {
		got := codes(obsStoryIssues([]string{"01"}, []obsStory{{num: "02", hasTitle: true, frames: 1, hasBibleRef: true}}))
		assert.Contains(t, got, repo_model.IssueCodeOBSStoryMissing)
		assert.Contains(t, got, repo_model.IssueCodeOBSWrongFrameCount)
	})

	t.Run("story with too few frames is an error", func(t *testing.T) {
		short := complete
		short.num, short.frames = "03", minOBSFrames-1
		first := complete
		first.num = "01"
		issues := obsStoryIssues(nil, []obsStory{first, short})
		assert.Equal(t, map[repo_model.IssueCode]repo_model.SeverityLevel{
			repo_model.IssueCodeOBSWrongFrameCount: repo_model.SeverityLevelError,
		}, codes(issues))
		assert.Contains(t, issues[0].Details, "fewer than 5 frames: **`03`**")
	})

	t.Run("missing Bible reference is a warning", func(t *testing.T) {
		noRef := complete
		noRef.num, noRef.hasBibleRef = "01", false
		assert.Equal(t, map[repo_model.IssueCode]repo_model.SeverityLevel{
			repo_model.IssueCodeOBSBibleRefenceMissing: repo_model.SeverityLevelWarning,
		}, codes(obsStoryIssues(nil, []obsStory{noRef})))
	})
}
