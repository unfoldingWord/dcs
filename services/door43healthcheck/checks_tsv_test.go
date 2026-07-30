// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tsvIssueCodesOf(issues []*repo_model.Door43HealthcheckIssue) []repo_model.IssueCode {
	codes := make([]repo_model.IssueCode, 0, len(issues))
	for _, issue := range issues {
		codes = append(codes, issue.IssueCode)
	}
	return codes
}

func TestCheckTSVFileContent(t *testing.T) {
	tnDM := &repo_model.Door43Metadata{Subject: "TSV Translation Notes", MetadataType: "rc"}
	twlDM := &repo_model.Door43Metadata{Subject: "TSV Translation Words Links", MetadataType: "rc"}
	tnHeader := "Reference\tID\tTags\tSupportReference\tQuote\tOccurrence\tNote"

	t.Run("valid TN file", func(t *testing.T) {
		content := tnHeader + "\n" +
			"front:intro\tabc1\t\t\t\t0\tAn intro note\n" +
			"1:1\tabc2\t\trc://*/ta/man/translate/figs-metaphor\tword\t1\tA note\n" +
			"1:2-3\tabc3\t\t\t\t0\tAnother note\n" +
			"1:4;1:6\tabc4\t\t\t\t0\tList reference\n"
		assert.Empty(t, checkTSVFileContent(tnDM, "tn_GEN.tsv", strings.NewReader(content)))
	})

	t.Run("column count, IDs, reference, occurrence, link", func(t *testing.T) {
		content := tnHeader + "\n" +
			"1:1\tabc1\t\t\tword\t1\tgood row\n" +
			"1:2\tabc1\t\t\t\t0\tduplicate id\n" + // TSV-005
			"1:3\tAB!\t\t\t\t0\tbad id\n" + // TSV-004
			"one:1\tabc4\t\t\t\t0\tbad reference\n" + // TSV-003
			"1:5\tabc5\t\t\tword\tX\tbad occurrence\n" + // TSV-006
			"1:6\tabc6\t\t\t\t2\tempty quote needs occurrence 0\n" + // TSV-006
			"1:7\tabc7\t\tfigs-metaphor\t\t0\tbare slug support ref\n" + // TSV-009
			"1:8\tabc8\ttoo\tfew\tcolumns\n" // TSV-002
		codes := tsvIssueCodesOf(checkTSVFileContent(tnDM, "tn_GEN.tsv", strings.NewReader(content)))
		assert.Contains(t, codes, repo_model.IssueCodeTSVIDDuplicate)
		assert.Contains(t, codes, repo_model.IssueCodeTSVIDInvalid)
		assert.Contains(t, codes, repo_model.IssueCodeTSVReferenceInvalid)
		assert.Contains(t, codes, repo_model.IssueCodeTSVOccurrenceInvalid)
		assert.Contains(t, codes, repo_model.IssueCodeTSVLinkInvalid)
		assert.Contains(t, codes, repo_model.IssueCodeTSVRowInvalid)
		assert.NotContains(t, codes, repo_model.IssueCodeTSVHeaderInvalid)
	})

	t.Run("TWL links and empty required cell", func(t *testing.T) {
		content := "Reference\tID\tTags\tOrigWords\tOccurrence\tTWLink\n" +
			"1:1\tabc1\tkeyterm\tword\t1\trc://*/tw/dict/bible/kt/god\n" +
			"1:2\tabc2\t\tword\t1\thttps://example.com/not-an-rc-link\n" + // TSV-008
			"1:3\tabc3\t\tword\t1\t\n" // TSV-011 (empty TWLink)
		issues := checkTSVFileContent(twlDM, "twl_GEN.tsv", strings.NewReader(content))
		codes := tsvIssueCodesOf(issues)
		assert.Contains(t, codes, repo_model.IssueCodeTSVLinkInvalid)
		assert.Contains(t, codes, repo_model.IssueCodeTSVCellEmpty)
		// the empty-cell finding is a Warning, the malformed link an Error
		for _, issue := range issues {
			if issue.IssueCode == repo_model.IssueCodeTSVCellEmpty {
				assert.Equal(t, repo_model.SeverityLevelWarning, issue.SeverityLevel)
			}
			if issue.IssueCode == repo_model.IssueCodeTSVLinkInvalid {
				assert.Equal(t, repo_model.SeverityLevelError, issue.SeverityLevel)
				assert.Equal(t, "TSV-008", issue.Rule)
			}
		}
	})

	t.Run("wrong header for the subject", func(t *testing.T) {
		content := "Ref\tID\tNote\n1:1\tabc1\tnote\n"
		codes := tsvIssueCodesOf(checkTSVFileContent(tnDM, "tn_GEN.tsv", strings.NewReader(content)))
		assert.Contains(t, codes, repo_model.IssueCodeTSVHeaderInvalid)
	})

	t.Run("legacy 9-column TN file is tolerated", func(t *testing.T) {
		content := "Book\tChapter\tVerse\tID\tSupportReference\tOrigQuote\tOccurrence\tGLQuote\tOccurrenceNote\n" +
			"GEN\t1\t1\tabc1\tfigs-metaphor\tword\t1\tgloss\ta note\n" // bare slug OK in legacy mode
		assert.Empty(t, checkTSVFileContent(tnDM, "en_tn_01-GEN.tsv", strings.NewReader(content)))
	})

	t.Run("OBS title-row reference {s}:0 is valid", func(t *testing.T) {
		assert.True(t, tsvReferenceValid("1:0"))
		assert.True(t, tsvReferenceValid("50:17"))
		assert.False(t, tsvReferenceValid("intro"))
		assert.False(t, tsvReferenceValid("1:1-"))
		assert.False(t, tsvReferenceValid(""))
	})

	t.Run("chapter front matter reference {c}:front is valid", func(t *testing.T) {
		// Psalm descriptions before verse 1, e.g. tn_PSA.tsv
		assert.True(t, tsvReferenceValid("119:front"))
		assert.True(t, tsvReferenceValid("3:front"))
		assert.True(t, tsvReferenceValid("front:intro;1:front,1:1"))
		assert.False(t, tsvReferenceValid("front:1"))
		assert.False(t, tsvReferenceValid("front:front"))
	})
}

func TestCheckTSVRowListCap(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("Reference\tID\tTags\tSupportReference\tQuote\tOccurrence\tNote\n")
	for range 25 {
		sb.WriteString("bad reference\n") // wrong column count on every row
	}
	dm := &repo_model.Door43Metadata{Subject: "TSV Translation Notes", MetadataType: "rc"}
	issues := checkTSVFileContent(dm, "tn_GEN.tsv", strings.NewReader(sb.String()))
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Details, "25 rows")
	assert.Contains(t, issues[0].Details, "and 15 more")
}
