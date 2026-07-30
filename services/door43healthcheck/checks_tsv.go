// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
)

// tsvHeadersBySubject are the exact 7-column TSV header schemas per subject (spec
// Appendix B). Subjects not listed are validated in generic mode (self-consistent
// column counts, ID and Occurrence rules) without an exact-header requirement.
var tsvHeadersBySubject = map[string][]string{
	"TSV Translation Notes":           {"Reference", "ID", "Tags", "SupportReference", "Quote", "Occurrence", "Note"},
	"TSV OBS Translation Notes":       {"Reference", "ID", "Tags", "SupportReference", "Quote", "Occurrence", "Note"},
	"TSV Translation Questions":       {"Reference", "ID", "Tags", "Quote", "Occurrence", "Question", "Response"},
	"TSV OBS Translation Questions":   {"Reference", "ID", "Tags", "Quote", "Occurrence", "Question", "Response"},
	"TSV OBS Study Questions":         {"Reference", "ID", "Tags", "Quote", "Occurrence", "Question", "Response"},
	"TSV Translation Words Links":     {"Reference", "ID", "Tags", "OrigWords", "Occurrence", "TWLink"},
	"TSV OBS Translation Words Links": {"Reference", "ID", "Tags", "OrigWords", "Occurrence", "TWLink"},
	"TSV OBS Study Notes":             {"Reference", "ID", "Tags", "Quote", "Occurrence", "Note"},
}

// tsv9Header is the legacy 9-column TN format; files carrying it are validated in
// generic mode without an exact-header Error (legacy content is frozen, spec §7.6/D8)
var tsv9Header = []string{"Book", "Chapter", "Verse", "ID", "SupportReference", "OrigQuote", "Occurrence", "GLQuote", "OccurrenceNote"}

var (
	tsvIDRegex              = regexp.MustCompile(`^[a-z][a-z0-9]{3}$`)                                                                // TSV-004
	tsvRefPartRegex         = regexp.MustCompile(`^(front:intro|\d+:intro|\d+:front)$`)                                               // TSV-003 (non-verse anchors)
	tsvRefChapterVerseRegex = regexp.MustCompile(`^\d+:\d+(-\d+)?$`)                                                                  // TSV-003 ({c}:{v} or {c}:{v}-{v2})
	tsvRefVerseRegex        = regexp.MustCompile(`^\d+(-\d+)?$`)                                                                      // TSV-003 (bare verse continuing the last chapter)
	twLinkRegex             = regexp.MustCompile(`^rc://(\*|[A-Za-z0-9-]+)/tw/dict/bible/(kt|names|other)/[A-Za-z0-9-]+$`)            // TSV-008
	taLinkRegex             = regexp.MustCompile(`^rc://(\*|[A-Za-z0-9-]+)/ta/man/(intro|process|translate|checking)/[A-Za-z0-9-]+$`) // TSV-009
)

// checkTSVFiles validates every TSV ingredient of a TSV-subject entry (rc and sb) per
// the TSV rules of the DCS Resource Validation Specification (TSV-001…TSV-011).
func checkTSVFiles(ctx context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if !repo_model.IsTSVSubject(dm.Subject) || len(dm.Ingredients) == 0 {
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
			!strings.HasSuffix(strings.ToLower(ingredient.Path), ".tsv") {
			continue // missing/empty files are flagged by the ingredient checks
		}
		blob, err := commit.GetBlobByPath(ingredient.Path)
		if err != nil || blob == nil {
			continue
		}
		dataRc, err := blob.DataAsync()
		if err != nil {
			log.Error("checkTSVFiles: DataAsync %s/%s: %v", dm.Repo.FullName(), ingredient.Path, err)
			continue
		}
		fileIssues := checkTSVFileContent(dm, ingredient.Path, dataRc)
		dataRc.Close()
		issues = append(issues, fileIssues...)
	}
	return issues
}

// tsvRowList accumulates offending row numbers for one violation type in one file
type tsvRowList []int

// String renders the row list capped at 10 rows
func (rl tsvRowList) String() string {
	shown := make([]string, 0, 10)
	for i, row := range rl {
		if i == 10 {
			break
		}
		shown = append(shown, strconv.Itoa(row))
	}
	s := "rows " + strings.Join(shown, ", ")
	if len(rl) == 1 {
		s = "row " + shown[0]
	}
	if len(rl) > 10 {
		s += fmt.Sprintf(" and %d more", len(rl)-10)
	}
	return s
}

// checkTSVFileContent validates one TSV file's content, streaming it line by line
func checkTSVFileContent(dm *repo_model.Door43Metadata, path string, r io.Reader) []*repo_model.Door43HealthcheckIssue {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	if !scanner.Scan() {
		return nil // empty file is flagged by the ingredient checks
	}
	header := strings.Split(strings.TrimPrefix(scanner.Text(), "\ufeff"), "\t")

	var issues []*repo_model.Door43HealthcheckIssue
	addIssue := func(code repo_model.IssueCode, severity repo_model.SeverityLevel, rule, details string) {
		issue := newIssue(code, severity,
			fmt.Sprintf(code.IssueDetailsFormatString(), details),
			code.IssueSuggestionFormatString())
		if rule != "" {
			issue.Rule = rule
		}
		issues = append(issues, issue)
	}

	// Strict mode: the file follows its subject's 7-column schema. Legacy 9-column TN
	// files get generic checks only; a file matching neither gets a TSV-001 Error and
	// then generic checks against its own header (self-consistency still matters).
	expected := tsvHeadersBySubject[dm.Subject]
	strict := expected != nil && slices.Equal(header, expected)
	if expected != nil && !strict && !slices.Equal(header, tsv9Header) {
		addIssue(repo_model.IssueCodeTSVHeaderInvalid, repo_model.SeverityLevelError, "",
			fmt.Sprintf("**`%s`**: the header row is `%s` but this resource type requires exactly `%s`",
				path, strings.Join(header, " | "), strings.Join(expected, " | ")))
	}

	colIdx := func(name string) int { return slices.Index(header, name) }
	idIdx := colIdx("ID")
	refIdx := colIdx("Reference")
	occIdx := colIdx("Occurrence")
	quoteIdx := colIdx("Quote")
	if quoteIdx < 0 {
		quoteIdx = colIdx("OrigWords")
	}
	if quoteIdx < 0 {
		quoteIdx = colIdx("OrigQuote") // legacy 9-column TN
	}
	twLinkIdx := colIdx("TWLink")
	supportRefIdx := colIdx("SupportReference")

	// The subject's primary content column (TSV-011): Note for notes, Question for
	// questions, TWLink for word links — only enforced in strict mode.
	contentIdx := -1
	if strict {
		for _, name := range []string{"Note", "Question", "TWLink"} {
			if idx := colIdx(name); idx >= 0 {
				contentIdx = idx
				break
			}
		}
	}

	var badColumnRows, badIDRows, dupIDRows, badRefRows, badOccRows, badLinkRows, badTALinkRows, emptyContentRows tsvRowList
	seenIDs := make(map[string]bool)

	rowNum := 1 // the header is row 1
	for scanner.Scan() {
		rowNum++
		line := scanner.Text()
		cells := strings.Split(line, "\t")

		if len(cells) != len(header) {
			badColumnRows = append(badColumnRows, rowNum)
			continue // cell positions can't be trusted on a malformed row
		}

		if idIdx >= 0 {
			id := cells[idIdx]
			if !tsvIDRegex.MatchString(id) {
				badIDRows = append(badIDRows, rowNum)
			} else if seenIDs[id] {
				dupIDRows = append(dupIDRows, rowNum)
			} else {
				seenIDs[id] = true
			}
		}

		if refIdx >= 0 && strict && !tsvReferenceValid(cells[refIdx]) {
			badRefRows = append(badRefRows, rowNum)
		}

		if occIdx >= 0 {
			// TSV-006: with Quote/OrigWords text, Occurrence is an integer >= -1; with an
			// empty Quote — intro rows like front:intro / {c}:intro — there is nothing to
			// count occurrences of, so Occurrence is 0 or blank
			occStr := strings.TrimSpace(cells[occIdx])
			if quoteIdx >= 0 && strings.TrimSpace(cells[quoteIdx]) == "" {
				if occStr != "" && occStr != "0" {
					badOccRows = append(badOccRows, rowNum)
				}
			} else if occ, err := strconv.Atoi(occStr); err != nil || occ < -1 {
				badOccRows = append(badOccRows, rowNum)
			}
		}

		if twLinkIdx >= 0 && strict {
			if link := strings.TrimSpace(cells[twLinkIdx]); link != "" && !twLinkRegex.MatchString(link) {
				badLinkRows = append(badLinkRows, rowNum)
			}
		}
		if supportRefIdx >= 0 && strict {
			if link := strings.TrimSpace(cells[supportRefIdx]); link != "" && !taLinkRegex.MatchString(link) {
				badTALinkRows = append(badTALinkRows, rowNum)
			}
		}

		if contentIdx >= 0 && strings.TrimSpace(cells[contentIdx]) == "" {
			emptyContentRows = append(emptyContentRows, rowNum)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Error("checkTSVFileContent: scanning %s/%s: %v", dm.Repo.FullName(), path, err)
	}

	if len(badColumnRows) > 0 {
		addIssue(repo_model.IssueCodeTSVRowInvalid, repo_model.SeverityLevelError, "",
			fmt.Sprintf("**`%s`**: %d rows do not have exactly %d tab-separated columns (%s)", path, len(badColumnRows), len(header), badColumnRows))
	}
	if len(badIDRows) > 0 {
		addIssue(repo_model.IssueCodeTSVIDInvalid, repo_model.SeverityLevelError, "",
			fmt.Sprintf("**`%s`**: %d rows have an ID not matching `^[a-z][a-z0-9]{3}$` (%s)", path, len(badIDRows), badIDRows))
	}
	if len(dupIDRows) > 0 {
		addIssue(repo_model.IssueCodeTSVIDDuplicate, repo_model.SeverityLevelError, "",
			fmt.Sprintf("**`%s`**: %d rows reuse an ID already used earlier in the file (%s)", path, len(dupIDRows), dupIDRows))
	}
	if len(badRefRows) > 0 {
		addIssue(repo_model.IssueCodeTSVReferenceInvalid, repo_model.SeverityLevelError, "",
			fmt.Sprintf("**`%s`**: %d rows have an invalid Reference (%s)", path, len(badRefRows), badRefRows))
	}
	if len(badOccRows) > 0 {
		addIssue(repo_model.IssueCodeTSVOccurrenceInvalid, repo_model.SeverityLevelError, "",
			fmt.Sprintf("**`%s`**: %d rows have an invalid Occurrence (%s)", path, len(badOccRows), badOccRows))
	}
	if len(badLinkRows) > 0 {
		addIssue(repo_model.IssueCodeTSVLinkInvalid, repo_model.SeverityLevelError, "TSV-008",
			fmt.Sprintf("**`%s`**: %d rows have a TWLink not matching the rc:// TW link grammar (%s)", path, len(badLinkRows), badLinkRows))
	}
	if len(badTALinkRows) > 0 {
		addIssue(repo_model.IssueCodeTSVLinkInvalid, repo_model.SeverityLevelError, "TSV-009",
			fmt.Sprintf("**`%s`**: %d rows have a SupportReference not matching the rc:// TA link grammar (%s)", path, len(badTALinkRows), badTALinkRows))
	}
	if len(emptyContentRows) > 0 {
		addIssue(repo_model.IssueCodeTSVCellEmpty, repo_model.SeverityLevelWarning, "",
			fmt.Sprintf("**`%s`**: %d rows have an empty **`%s`** cell (%s)", path, len(emptyContentRows), header[contentIdx], emptyContentRows))
	}

	return issues
}

// tsvReferenceValid checks a Reference cell against the reference grammar (TSV-003):
// front:intro, {c}:intro, {c}:front, {c}:{v}, {c}:{v}-{v2}, or semicolon/comma lists
// thereof (spaces allowed around separators). Once a {c}:{v} segment names a chapter,
// a bare {v} or {v}-{v2} segment continues that chapter — common compound Bible
// references like 5:1,3,8,12 or 5:13-14,6:1-2. {c}:front anchors chapter front matter
// before verse 1 (Psalm descriptions in tn_PSA.tsv). Verse 0 is allowed (the OBS
// title-row convention {story}:0).
func tsvReferenceValid(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	inChapter := false // a {c}:{v} segment has named the chapter bare verses continue
	for _, part := range strings.FieldsFunc(ref, func(r rune) bool { return r == ';' || r == ',' }) {
		part = strings.TrimSpace(part)
		switch {
		case tsvRefChapterVerseRegex.MatchString(part):
			inChapter = true
		case tsvRefPartRegex.MatchString(part):
			// front:intro / {c}:intro / {c}:front anchor no verse to continue from
		case inChapter && tsvRefVerseRegex.MatchString(part):
			// bare verse (range) in the last named chapter
		default:
			return false
		}
	}
	return true
}
