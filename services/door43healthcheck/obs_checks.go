// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
)

var (
	storyTitleRegex     = regexp.MustCompile(`^#\s+\S`)
	bibleReferenceRegex = regexp.MustCompile(`^_.*_\s*$`)
	frameImageRegex     = regexp.MustCompile(`!\[`)
)

// minOBSFrames is the fewest frames a complete story may have; the shortest English stories have 7
const minOBSFrames = 5

// obsStory is the analysis of one existing story file
type obsStory struct {
	num         string
	hasTitle    bool
	frames      int
	hasBibleRef bool
}

// CheckOBSStories checks that all 50 OBS story files exist and have valid content.
// It verifies: file existence, story titles, a minimum frame count, and Bible references.
func CheckOBSStories(ctx context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Subject != "Open Bible Stories" {
		return nil
	}

	_ = dm.LoadRepo(ctx)

	// Find the content path from the OBS ingredient
	contentPath := findOBSContentPath(dm)
	if contentPath == "" {
		// No ingredient found; the ingredient check will already flag this
		return nil
	}

	// Open the git repo and get the commit
	gitRepo, commit := openCommit(ctx, dm)
	if commit == nil {
		return nil
	}
	defer gitRepo.Close()

	// Burritos converted from RCs keep the stories in a content/ dir under the ingredient dir
	if entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, path.Join(contentPath, "content")); err == nil && entry.IsDir() {
		contentPath = path.Join(contentPath, "content")
	}

	var missingStories []string
	var stories []obsStory

	for i := 1; i <= 50; i++ {
		storyNum := fmt.Sprintf("%02d", i)
		storyFile := path.Join(contentPath, storyNum+".md")

		blob, err := commit.GetBlobByPath(ctx, gitRepo, storyFile)
		if err != nil || blob == nil {
			missingStories = append(missingStories, storyNum)
			continue
		}

		// Read the file content to check title, frames, and Bible reference
		dataRc, err := blob.DataAsync(ctx)
		if err != nil {
			log.Error("CheckOBSStories: DataAsync Error for %s: %v", storyFile, err)
			continue
		}

		story := analyzeOBSStory(dataRc)
		dataRc.Close()
		story.num = storyNum
		stories = append(stories, story)
	}

	return obsStoryIssues(missingStories, stories)
}

// isOBSPlaceholder reports whether the stories are just 01 with at most one frame, which
// stands in for an audio/video-only OBS (e.g. Door43-Catalog/ylb_obs) and so is not checked
func isOBSPlaceholder(stories []obsStory) bool {
	return len(stories) == 1 && stories[0].num == "01" && stories[0].frames <= 1
}

// obsStoryIssues builds the issues for the missing and existing stories. Severities per
// the DCS Resource Validation Specification: a missing story (COMP-020) and a story with
// no title or too few frames (MD-002) are Errors; a missing final Bible-reference line
// is a Warning (MD-002).
func obsStoryIssues(missingStories []string, stories []obsStory) []*repo_model.Door43HealthcheckIssue {
	if isOBSPlaceholder(stories) {
		return nil
	}

	var missingTitles []string
	var missingFrames []string
	var missingBibleRefs []string
	for _, story := range stories {
		if !story.hasTitle {
			missingTitles = append(missingTitles, story.num)
		}
		if story.frames < minOBSFrames {
			missingFrames = append(missingFrames, story.num)
		}
		if !story.hasBibleRef {
			missingBibleRefs = append(missingBibleRefs, story.num)
		}
	}

	var issues []*repo_model.Door43HealthcheckIssue

	if len(missingStories) > 0 {
		issues = append(issues, newIssue(repo_model.IssueCodeOBSStoryMissing, repo_model.SeverityLevelError,
			fmt.Sprintf(repo_model.IssueCodeOBSStoryMissing.IssueDetailsFormatString(), strings.Join(missingStories, ", ")),
			fmt.Sprintf(repo_model.IssueCodeOBSStoryMissing.IssueSuggestionFormatString(), strings.Join(missingStories, ", "))))
	}

	if len(missingTitles) > 0 {
		issues = append(issues, newIssue(repo_model.IssueCodeOBSStoryTitleMissing, repo_model.SeverityLevelError,
			fmt.Sprintf(repo_model.IssueCodeOBSStoryTitleMissing.IssueDetailsFormatString(), strings.Join(missingTitles, ", ")),
			fmt.Sprintf(repo_model.IssueCodeOBSStoryTitleMissing.IssueSuggestionFormatString(), strings.Join(missingTitles, ", "))))
	}

	if len(missingFrames) > 0 {
		issues = append(issues, newIssue(repo_model.IssueCodeOBSWrongFrameCount, repo_model.SeverityLevelError,
			fmt.Sprintf(repo_model.IssueCodeOBSWrongFrameCount.IssueDetailsFormatString(), minOBSFrames, strings.Join(missingFrames, ", ")),
			fmt.Sprintf(repo_model.IssueCodeOBSWrongFrameCount.IssueSuggestionFormatString(), strings.Join(missingFrames, ", "))))
	}

	if len(missingBibleRefs) > 0 {
		issues = append(issues, newIssue(repo_model.IssueCodeOBSBibleRefenceMissing, repo_model.SeverityLevelWarning,
			fmt.Sprintf(repo_model.IssueCodeOBSBibleRefenceMissing.IssueDetailsFormatString(), strings.Join(missingBibleRefs, ", ")),
			fmt.Sprintf(repo_model.IssueCodeOBSBibleRefenceMissing.IssueSuggestionFormatString(), strings.Join(missingBibleRefs, ", "))))
	}

	return issues
}

// findOBSContentPath returns the content path for OBS stories from the manifest ingredients.
// For OBS, there's typically one ingredient with identifier "obs" and a path like "./content".
func findOBSContentPath(dm *repo_model.Door43Metadata) string {
	for _, ingredient := range dm.Ingredients {
		if ingredient.Identifier == "obs" {
			p := strings.TrimPrefix(ingredient.Path, "./")
			if p == "" {
				p = "."
			}
			return p
		}
	}
	// Fallback: if the only ingredient is a directory, use it
	if len(dm.Ingredients) == 1 && dm.Ingredients[0].IsDir {
		p := strings.TrimPrefix(dm.Ingredients[0].Path, "./")
		if p == "" {
			p = "."
		}
		return p
	}
	return ""
}

// analyzeOBSStory reads an OBS story file and checks for a title, its frames, and a Bible
// reference. The checks are language-agnostic:
//   - title: the first line of the file is a heading ("# ...") followed by a blank line
//   - frames: the number of lines with an image ("![")
//   - Bible reference: the last non-blank line is italicized ("_..._") and preceded by a
//     blank line, with only blank lines allowed after it
func analyzeOBSStory(r io.Reader) (story obsStory) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Error("analyzeOBSStory: scan error: %v", err)
		return story
	}
	if len(lines) == 0 {
		return story
	}
	lines[0] = strings.TrimPrefix(lines[0], "\ufeff") // ignore a UTF-8 BOM

	story.hasTitle = len(lines) >= 2 && storyTitleRegex.MatchString(lines[0]) && strings.TrimSpace(lines[1]) == ""

	for _, line := range lines {
		if frameImageRegex.MatchString(line) {
			story.frames++
		}
	}

	last := len(lines) - 1
	for last >= 0 && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	story.hasBibleRef = last >= 1 && bibleReferenceRegex.MatchString(lines[last]) && strings.TrimSpace(lines[last-1]) == ""

	return story
}
