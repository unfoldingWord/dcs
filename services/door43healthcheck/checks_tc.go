// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"context"
	"fmt"
	"io"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
)

// Checks in this file only run for translationCore (tc) repos.

// checkTcUSFM verifies the tc repo's USFM file is structurally valid USFM (leading \id marker
// matching the manifest's book, chapter and verse markers) and warns when it contains no
// alignment data. Missing alignment is a warning for now; it may become an error later.
func checkTcUSFM(ctx context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if len(dm.Ingredients) == 0 {
		return nil
	}
	ingredient := dm.Ingredients[0]
	if !ingredient.Exists || ingredient.Size == 0 {
		return nil // the ingredient checks already flag a missing or empty file
	}

	gitRepo, commit := openCommit(ctx, dm)
	if commit == nil {
		return nil
	}
	defer gitRepo.Close()

	blob, err := commit.GetBlobByPath(ingredient.Path)
	if err != nil || blob == nil {
		return nil // flagged by the ingredient checks
	}
	dataRc, err := blob.DataAsync()
	if err != nil {
		log.Error("checkTcUSFM: DataAsync %s/%s: %v", dm.Repo.FullName(), ingredient.Path, err)
		return nil
	}
	buf, err := io.ReadAll(dataRc)
	dataRc.Close()
	if err != nil {
		log.Error("checkTcUSFM: reading %s/%s: %v", dm.Repo.FullName(), ingredient.Path, err)
		return nil
	}
	content := string(buf)

	var issues []*repo_model.Door43HealthcheckIssue
	invalid := func(reason string) {
		issues = append(issues, newIssue(repo_model.IssueCodeUSFMInvalid, repo_model.SeverityLevelError,
			fmt.Sprintf(repo_model.IssueCodeUSFMInvalid.IssueDetailsFormatString(), ingredient.Path, reason),
			fmt.Sprintf(repo_model.IssueCodeUSFMInvalid.IssueSuggestionFormatString(), strings.ToUpper(ingredient.Identifier), ingredient.Path)))
	}

	bookID := usfmBookID(content)
	if bookID == "" {
		invalid("does not start with an **`\\id`** marker so is not valid USFM")
	} else if !strings.EqualFold(bookID, ingredient.Identifier) {
		invalid(fmt.Sprintf("has the book ID **`%s`** but the manifest expects **`%s`**", bookID, strings.ToUpper(ingredient.Identifier)))
	}
	if !strings.Contains(content, "\\c ") {
		invalid("does not contain any chapter (**`\\c`**) markers")
	}
	if !strings.Contains(content, "\\v ") {
		invalid("does not contain any verse (**`\\v`**) markers")
	}

	if !strings.Contains(content, "\\zaln-s") {
		issues = append(issues, newIssue(repo_model.IssueCodeUSFMNoAlignment, repo_model.SeverityLevelWarning,
			fmt.Sprintf(repo_model.IssueCodeUSFMNoAlignment.IssueDetailsFormatString(), ingredient.Path),
			repo_model.IssueCodeUSFMNoAlignment.IssueSuggestionFormatString()))
	}

	return issues
}

// usfmBookID returns the book code from the file's leading \id marker, or "" if absent
func usfmBookID(content string) string {
	content = strings.TrimLeft(strings.TrimPrefix(content, "\ufeff"), " \t\r\n")
	if !strings.HasPrefix(content, "\\id ") {
		return ""
	}
	line := content[len("\\id "):]
	if idx := strings.IndexAny(line, "\r\n"); idx >= 0 {
		line = line[:idx]
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
