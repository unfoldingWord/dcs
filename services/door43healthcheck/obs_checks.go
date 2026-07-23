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
	"slices"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
)

var (
	storyTitleRegex     = regexp.MustCompile(`^#\s+\S`)
	bibleReferenceRegex = regexp.MustCompile(`^_.*_\s*$`)
	frameImageRegex     = regexp.MustCompile(`!\[`)
)

// CheckOBSStories checks that all 50 OBS story files exist and have valid content.
// It verifies: file existence, story titles, at least one frame/image, and Bible references.
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
	gitRepo, err := git.OpenRepository(ctx, dm.Repo.RepoPath())
	if err != nil {
		log.Error("CheckOBSStories: OpenRepository Error %s: %v", dm.Repo.FullName(), err)
		return nil
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(dm.CommitSHA)
	if err != nil {
		log.Error("CheckOBSStories: GetCommit Error %s/%s: %v", dm.Repo.FullName(), dm.CommitSHA, err)
		return nil
	}

	var missingStories []string
	var missingTitles []string
	var missingFrames []string
	var missingBibleRefs []string

	for i := 1; i <= 50; i++ {
		storyNum := fmt.Sprintf("%02d", i)
		storyFile := path.Join(contentPath, storyNum+".md")

		blob, err := commit.GetBlobByPath(storyFile)
		if err != nil || blob == nil {
			missingStories = append(missingStories, storyNum)
			continue
		}

		// Read the file content to check title, frames, and Bible reference
		dataRc, err := blob.DataAsync()
		if err != nil {
			log.Error("CheckOBSStories: DataAsync Error for %s: %v", storyFile, err)
			continue
		}

		hasTitle, hasFrame, hasBibleRef := analyzeOBSStory(dataRc)
		dataRc.Close()

		if !hasTitle {
			missingTitles = append(missingTitles, storyNum)
		}
		if !hasFrame {
			missingFrames = append(missingFrames, storyNum)
		}
		if !hasBibleRef {
			missingBibleRefs = append(missingBibleRefs, storyNum)
		}
	}

	var issues []*repo_model.Door43HealthcheckIssue

	if len(missingStories) > 0 {
		issues = append(issues, &repo_model.Door43HealthcheckIssue{
			IssueCode:     repo_model.IssueCodeOBSStoryMissing,
			SeverityLevel: repo_model.SeverityLevelError,
			PositiveTitle: repo_model.IssueCodeOBSStoryMissing.IssuePositiveString(),
			NegativeTitle: repo_model.IssueCodeOBSStoryMissing.IssueNegativeString(),
			Details:       fmt.Sprintf(repo_model.IssueCodeOBSStoryMissing.IssueDetailsFormatString(), strings.Join(missingStories, ", ")),
			Suggestion:    fmt.Sprintf(repo_model.IssueCodeOBSStoryMissing.IssueSuggestionFormatString(), strings.Join(missingStories, ", ")),
		})
	}

	if len(missingTitles) > 0 {
		issues = append(issues, &repo_model.Door43HealthcheckIssue{
			IssueCode:     repo_model.IssueCodeOBSStoryTitleMissing,
			SeverityLevel: repo_model.SeverityLevelWarning,
			PositiveTitle: repo_model.IssueCodeOBSStoryTitleMissing.IssuePositiveString(),
			NegativeTitle: repo_model.IssueCodeOBSStoryTitleMissing.IssueNegativeString(),
			Details:       fmt.Sprintf(repo_model.IssueCodeOBSStoryTitleMissing.IssueDetailsFormatString(), strings.Join(missingTitles, ", ")),
			Suggestion:    fmt.Sprintf(repo_model.IssueCodeOBSStoryTitleMissing.IssueSuggestionFormatString(), strings.Join(missingTitles, ", ")),
		})
	}

	if len(missingFrames) > 0 {
		issues = append(issues, &repo_model.Door43HealthcheckIssue{
			IssueCode:     repo_model.IssueCodeOBSWrongFrameCount,
			SeverityLevel: repo_model.SeverityLevelWarning,
			PositiveTitle: repo_model.IssueCodeOBSWrongFrameCount.IssuePositiveString(),
			NegativeTitle: repo_model.IssueCodeOBSWrongFrameCount.IssueNegativeString(),
			Details:       fmt.Sprintf(repo_model.IssueCodeOBSWrongFrameCount.IssueDetailsFormatString(), strings.Join(missingFrames, ", ")),
			Suggestion:    fmt.Sprintf(repo_model.IssueCodeOBSWrongFrameCount.IssueSuggestionFormatString(), strings.Join(missingFrames, ", ")),
		})
	}

	if len(missingBibleRefs) > 0 {
		issues = append(issues, &repo_model.Door43HealthcheckIssue{
			IssueCode:     repo_model.IssueCodeOBSBibleRefenceMissing,
			SeverityLevel: repo_model.SeverityLevelWarning,
			PositiveTitle: repo_model.IssueCodeOBSBibleRefenceMissing.IssuePositiveString(),
			NegativeTitle: repo_model.IssueCodeOBSBibleRefenceMissing.IssueNegativeString(),
			Details:       fmt.Sprintf(repo_model.IssueCodeOBSBibleRefenceMissing.IssueDetailsFormatString(), strings.Join(missingBibleRefs, ", ")),
			Suggestion:    fmt.Sprintf(repo_model.IssueCodeOBSBibleRefenceMissing.IssueSuggestionFormatString(), strings.Join(missingBibleRefs, ", ")),
		})
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

// analyzeOBSStory reads an OBS story file and checks for a title, at least one frame image,
// and a Bible reference. The checks are language-agnostic:
//   - title: the first line of the file is a heading ("# ...") followed by a blank line
//   - frame: at least one image ("![") anywhere in the file
//   - Bible reference: the last non-blank line is italicized ("_..._") and preceded by a
//     blank line, with only blank lines allowed after it
func analyzeOBSStory(r io.Reader) (hasTitle, hasFrame, hasBibleRef bool) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Error("analyzeOBSStory: scan error: %v", err)
		return false, false, false
	}
	if len(lines) == 0 {
		return false, false, false
	}
	lines[0] = strings.TrimPrefix(lines[0], "\ufeff") // ignore a UTF-8 BOM

	hasTitle = len(lines) >= 2 && storyTitleRegex.MatchString(lines[0]) && strings.TrimSpace(lines[1]) == ""

	hasFrame = slices.ContainsFunc(lines, frameImageRegex.MatchString)

	last := len(lines) - 1
	for last >= 0 && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	hasBibleRef = last >= 1 && bibleReferenceRegex.MatchString(lines[last]) && strings.TrimSpace(lines[last-1]) == ""

	return hasTitle, hasFrame, hasBibleRef
}
