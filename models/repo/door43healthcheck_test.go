// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoor43HealthcheckIssuePersistence(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	dm := &repo_model.Door43Metadata{
		RepoID: 1, Ref: "master", RefType: "branch",
		CommitSHA: "0000000000000000000000000000000000000031",
		Stage:     door43metadata.StageLatest,
		Language:  "en", Subject: "Open Bible Stories", MetadataType: "rc",
	}
	_, err := db.GetEngine(t.Context()).Insert(dm)
	require.NoError(t, err)

	issues := []*repo_model.Door43HealthcheckIssue{
		{IssueCode: repo_model.IssueCodeTitle, SeverityLevel: repo_model.SeverityLevelError, NegativeTitle: "t1", Details: "d1", Suggestion: "s1"},
		{IssueCode: repo_model.IssueCodeLanguage, SeverityLevel: repo_model.SeverityLevelWarning, NegativeTitle: "t2", Details: "d2", Suggestion: "s2"},
	}
	require.NoError(t, repo_model.ReplaceDoor43HealthcheckIssues(t.Context(), dm, issues))

	stored, err := repo_model.GetDoor43HealthcheckIssuesByDMID(t.Context(), dm.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	// check order is preserved and the DM/repo keys were stamped on
	assert.Equal(t, repo_model.IssueCodeTitle, stored[0].IssueCode)
	assert.Equal(t, repo_model.IssueCodeLanguage, stored[1].IssueCode)
	assert.Equal(t, dm.ID, stored[0].DMID)
	assert.Equal(t, dm.RepoID, stored[0].RepoID)

	// a replace fully swaps the stored rows
	require.NoError(t, repo_model.ReplaceDoor43HealthcheckIssues(t.Context(), dm,
		[]*repo_model.Door43HealthcheckIssue{{IssueCode: repo_model.IssueCodeLanguage, SeverityLevel: repo_model.SeverityLevelWarning}}))
	stored, err = repo_model.GetDoor43HealthcheckIssuesByDMID(t.Context(), dm.ID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, repo_model.IssueCodeLanguage, stored[0].IssueCode)

	// LoadHealthcheck rebuilds the grouped issues from the stored rows
	hgi, err := dm.LoadHealthcheck(t.Context())
	require.NoError(t, err)
	assert.Equal(t, repo_model.SeverityLevelWarning, hgi.OverallSeverityLevel)
	assert.Equal(t, 1, hgi.SeverityLevelCount[repo_model.SeverityLevelWarning])

	// GetHealthcheck serves the stored rows when they add up to the stored severity
	dm.HealthcheckSeverity = repo_model.SeverityLevelWarning
	hgi = dm.GetHealthcheck(t.Context())
	require.NotNil(t, hgi)
	assert.Equal(t, repo_model.SeverityLevelWarning, hgi.OverallSeverityLevel)

	// deleting the DM removes its issues
	require.NoError(t, repo_model.DeleteDoor43Metadata(t.Context(), dm))
	stored, err = repo_model.GetDoor43HealthcheckIssuesByDMID(t.Context(), dm.ID)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

func TestIssueCodesFor(t *testing.T) {
	rcOBS := repo_model.IssueCodesFor("rc", "Open Bible Stories")
	assert.Contains(t, rcOBS, repo_model.IssueCodePublisher)
	assert.Contains(t, rcOBS, repo_model.IssueCodeOBSStoryMissing)

	tc := repo_model.IssueCodesFor("tc", "Aligned Bible")
	assert.Contains(t, tc, repo_model.IssueCodeUSFMInvalid)
	assert.Contains(t, tc, repo_model.IssueCodeUSFMNoAlignment)
	assert.NotContains(t, tc, repo_model.IssueCodePublisher)

	sb := repo_model.IssueCodesFor("sb", "Bible")
	assert.Contains(t, sb, repo_model.IssueCodeSBIngredientMismatch)
	assert.NotContains(t, sb, repo_model.IssueCodeOBSStoryMissing)

	ts := repo_model.IssueCodesFor("ts", "Bible")
	assert.Contains(t, ts, repo_model.IssueCodeIngredientMissing)
	assert.NotContains(t, ts, repo_model.IssueCodeUSFMInvalid)
}

func TestSeverityLevelIsHealthy(t *testing.T) {
	assert.True(t, repo_model.SeverityLevelSuccess.IsHealthy())
	assert.True(t, repo_model.SeverityLevelInfo.IsHealthy())
	assert.True(t, repo_model.SeverityLevelWarning.IsHealthy())
	assert.False(t, repo_model.SeverityLevelError.IsHealthy())
	assert.False(t, repo_model.SeverityLevel(0).IsHealthy()) // never checked

	assert.True(t, repo_model.SeverityLevelSuccess.IsHealthyWithoutWarnings())
	assert.True(t, repo_model.SeverityLevelInfo.IsHealthyWithoutWarnings())
	assert.False(t, repo_model.SeverityLevelWarning.IsHealthyWithoutWarnings())
	assert.False(t, repo_model.SeverityLevelError.IsHealthyWithoutWarnings())
	assert.False(t, repo_model.SeverityLevel(0).IsHealthyWithoutWarnings())
}
