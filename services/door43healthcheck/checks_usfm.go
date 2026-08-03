// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"context"
	"fmt"
	"io"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
)

// checkUSFMBooks verifies every USFM book ingredient of a scripture entry (rc, sb and tc
// Bibles) is structurally valid USFM: a leading \id marker matching the ingredient's book
// (USFM-001/USFM-002) and chapter/verse markers present (Errors). For Aligned Bibles, a
// book with no alignment data (\zaln-s) gets a Warning (USFM-009); missing alignment may
// become an Error later.
func checkUSFMBooks(ctx context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if !repo_model.IsScriptureSubject(dm.Subject) || len(dm.Ingredients) == 0 {
		return nil
	}

	gitRepo, commit := openCommit(ctx, dm)
	if commit == nil {
		return nil
	}
	defer gitRepo.Close()

	var issues []*repo_model.Door43HealthcheckIssue
	for _, ingredient := range dm.Ingredients {
		if !ingredient.Exists || ingredient.Size == 0 || ingredient.IsDir ||
			!strings.HasSuffix(strings.ToLower(ingredient.Path), ".usfm") ||
			ingredient.Identifier == "frt" || ingredient.Identifier == "bak" {
			continue // missing/empty files are flagged by the ingredient checks; frt/bak have no chapters
		}
		issues = append(issues, checkUSFMBook(commit, dm, ingredient.Path, ingredient.Identifier)...)
	}
	return issues
}

// checkUSFMBook validates one USFM book file at the given path
func checkUSFMBook(commit *git.Commit, dm *repo_model.Door43Metadata, path, identifier string) []*repo_model.Door43HealthcheckIssue {
	blob, err := commit.GetBlobByPath(path)
	if err != nil || blob == nil {
		return nil // flagged by the ingredient checks
	}
	dataRc, err := blob.DataAsync()
	if err != nil {
		log.Error("checkUSFMBook: DataAsync %s/%s: %v", dm.Repo.FullName(), path, err)
		return nil
	}
	buf, err := io.ReadAll(dataRc)
	dataRc.Close()
	if err != nil {
		log.Error("checkUSFMBook: reading %s/%s: %v", dm.Repo.FullName(), path, err)
		return nil
	}
	content := string(buf)

	var issues []*repo_model.Door43HealthcheckIssue
	invalid := func(rule, reason string) {
		issue := newIssue(repo_model.IssueCodeUSFMInvalid, repo_model.SeverityLevelError,
			fmt.Sprintf(repo_model.IssueCodeUSFMInvalid.IssueDetailsFormatString(), path, reason),
			fmt.Sprintf(repo_model.IssueCodeUSFMInvalid.IssueSuggestionFormatString(), strings.ToUpper(identifier), path))
		issue.Rule = rule
		issues = append(issues, issue)
	}

	bookID := usfmBookID(content)
	if bookID == "" {
		invalid("USFM-001", "does not start with an **`\\id`** marker so is not valid USFM")
	} else if !strings.EqualFold(bookID, identifier) {
		invalid("USFM-002", fmt.Sprintf("has the book ID **`%s`** but the metadata expects **`%s`**", bookID, strings.ToUpper(identifier)))
	}
	if !strings.Contains(content, "\\c ") {
		invalid("USFM-004", "does not contain any chapter (**`\\c`**) markers")
	}
	if !strings.Contains(content, "\\v ") {
		invalid("USFM-005", "does not contain any verse (**`\\v`**) markers")
	}

	if dm.Subject == "Aligned Bible" && !strings.Contains(content, "\\zaln-s") {
		issues = append(issues, newIssue(repo_model.IssueCodeUSFMNoAlignment, repo_model.SeverityLevelWarning,
			fmt.Sprintf(repo_model.IssueCodeUSFMNoAlignment.IssueDetailsFormatString(), path),
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
