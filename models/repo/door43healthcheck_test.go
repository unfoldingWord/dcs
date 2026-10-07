// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"

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

	dm.HealthcheckSeverity = repo_model.SeverityLevelError
	issues := []*repo_model.Door43HealthcheckIssue{
		{IssueCode: repo_model.IssueCodeTitle, SeverityLevel: repo_model.SeverityLevelError, NegativeTitle: "t1", Details: "d1", Suggestion: "s1"},
		{IssueCode: repo_model.IssueCodeLanguage, SeverityLevel: repo_model.SeverityLevelWarning, NegativeTitle: "t2", Details: "d2", Suggestion: "s2"},
	}
	storedOK, err := repo_model.StoreHealthcheckResults(t.Context(), dm, issues)
	require.NoError(t, err)
	assert.True(t, storedOK)

	stored, err := repo_model.GetDoor43HealthcheckIssuesByDMID(t.Context(), dm.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	// check order is preserved and the DM/repo keys were stamped on
	assert.Equal(t, repo_model.IssueCodeTitle, stored[0].IssueCode)
	assert.Equal(t, repo_model.IssueCodeLanguage, stored[1].IssueCode)
	assert.Equal(t, dm.ID, stored[0].DMID)
	assert.Equal(t, dm.RepoID, stored[0].RepoID)

	// a new store fully swaps the stored rows
	dm.HealthcheckSeverity = repo_model.SeverityLevelWarning
	storedOK, err = repo_model.StoreHealthcheckResults(t.Context(), dm,
		[]*repo_model.Door43HealthcheckIssue{{IssueCode: repo_model.IssueCodeLanguage, SeverityLevel: repo_model.SeverityLevelWarning}})
	require.NoError(t, err)
	assert.True(t, storedOK)
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
	hgi = dm.GetHealthcheck(t.Context())
	require.NotNil(t, hgi)
	assert.Equal(t, repo_model.SeverityLevelWarning, hgi.OverallSeverityLevel)

	// deleting the DM removes its issues
	require.NoError(t, repo_model.DeleteDoor43Metadata(t.Context(), dm))
	stored, err = repo_model.GetDoor43HealthcheckIssuesByDMID(t.Context(), dm.ID)
	require.NoError(t, err)
	assert.Empty(t, stored)

	// storing results for an entry deleted while its check ran writes nothing —
	// no orphaned severity update, no orphaned issue rows
	storedOK, err = repo_model.StoreHealthcheckResults(t.Context(), dm, issues)
	require.NoError(t, err)
	assert.False(t, storedOK)
	stored, err = repo_model.GetDoor43HealthcheckIssuesByDMID(t.Context(), dm.ID)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

func TestDeleteDoor43MetadatasStaleRefs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	newDM := func(ref, refType, sha string) *repo_model.Door43Metadata {
		dm := &repo_model.Door43Metadata{
			RepoID: 1, Ref: ref, RefType: refType, CommitSHA: sha,
			Stage: door43metadata.StageOther, Language: "en", Subject: "Bible", MetadataType: "rc",
		}
		_, err := db.GetEngine(t.Context()).Insert(dm)
		require.NoError(t, err)
		return dm
	}
	live1 := newDM("master", "branch", "0000000000000000000000000000000000000041")
	live2 := newDM("v1", "tag", "0000000000000000000000000000000000000042")
	stale := newDM("deleted-branch", "branch", "0000000000000000000000000000000000000043")
	_, err := repo_model.StoreHealthcheckResults(t.Context(), stale,
		[]*repo_model.Door43HealthcheckIssue{{IssueCode: repo_model.IssueCodeTitle, SeverityLevel: repo_model.SeverityLevelError}})
	require.NoError(t, err)

	// rows updated after olderThan are spared (e.g. a branch pushed mid-pass)
	count, err := repo_model.DeleteDoor43MetadatasStaleRefs(t.Context(), 1, []string{"master", "v1"}, timeutil.TimeStamp(1))
	require.NoError(t, err)
	assert.EqualValues(t, 0, count)

	// an empty ref list never deletes anything
	count, err = repo_model.DeleteDoor43MetadatasStaleRefs(t.Context(), 1, nil, timeutil.TimeStampNow()+10)
	require.NoError(t, err)
	assert.EqualValues(t, 0, count)

	// the stale ref's entry and its issues are swept; live refs stay
	count, err = repo_model.DeleteDoor43MetadatasStaleRefs(t.Context(), 1, []string{"master", "v1"}, timeutil.TimeStampNow()+10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
	issues, err := repo_model.GetDoor43HealthcheckIssuesByDMID(t.Context(), stale.ID)
	require.NoError(t, err)
	assert.Empty(t, issues)
	for _, id := range []int64{live1.ID, live2.ID} {
		exists, err := db.GetEngine(t.Context()).ID(id).Exist(new(repo_model.Door43Metadata))
		require.NoError(t, err)
		assert.True(t, exists)
	}
	staleExists, err := db.GetEngine(t.Context()).ID(stale.ID).Exist(new(repo_model.Door43Metadata))
	require.NoError(t, err)
	assert.False(t, staleExists)
}

func TestGetHealthcheckSeveritiesByRefs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for ref, severity := range map[string]repo_model.SeverityLevel{
		"badge-branch": repo_model.SeverityLevelWarning,
		"badge-v1":     0, // entry exists but was never checked
	} {
		dm := &repo_model.Door43Metadata{
			RepoID: 1, Ref: ref, RefType: "branch",
			CommitSHA: "0000000000000000000000000000000000000061",
			Stage:     door43metadata.StageOther, Language: "en", Subject: "Bible", MetadataType: "rc",
			HealthcheckSeverity: severity,
		}
		_, err := db.GetEngine(t.Context()).Insert(dm)
		require.NoError(t, err)
	}

	severities, err := repo_model.GetHealthcheckSeveritiesByRefs(t.Context(), 1, []string{"badge-branch", "badge-v1", "no-such-ref"})
	require.NoError(t, err)
	assert.Equal(t, repo_model.SeverityLevelWarning, severities["badge-branch"])
	assert.Contains(t, severities, "badge-v1") // present but zero severity: badge hidden by the template
	assert.NotContains(t, severities, "no-such-ref")

	severities, err = repo_model.GetHealthcheckSeveritiesByRefs(t.Context(), 1, nil)
	require.NoError(t, err)
	assert.Empty(t, severities)
}

func TestIssueCodesFor(t *testing.T) {
	rcOBS := repo_model.IssueCodesFor("rc", "Open Bible Stories")
	assert.Contains(t, rcOBS, repo_model.IssueCodePublisher)
	assert.Contains(t, rcOBS, repo_model.IssueCodeOBSStoryMissing)

	rcBible := repo_model.IssueCodesFor("rc", "Aligned Bible")
	assert.Contains(t, rcBible, repo_model.IssueCodeUSFMInvalid)
	assert.NotContains(t, rcBible, repo_model.IssueCodeTSVRowInvalid)

	rcTN := repo_model.IssueCodesFor("rc", "TSV Translation Notes")
	assert.Contains(t, rcTN, repo_model.IssueCodeTSVRowInvalid)
	assert.Contains(t, rcTN, repo_model.IssueCodeTSVIDDuplicate)
	assert.Contains(t, rcTN, repo_model.IssueCodeOrigLangVersionMissing)
	assert.NotContains(t, rcTN, repo_model.IssueCodeUSFMInvalid)

	tc := repo_model.IssueCodesFor("tc", "Aligned Bible")
	assert.Contains(t, tc, repo_model.IssueCodeUSFMInvalid)
	assert.Contains(t, tc, repo_model.IssueCodeUSFMNoAlignment)
	assert.NotContains(t, tc, repo_model.IssueCodePublisher)

	sb := repo_model.IssueCodesFor("sb", "Bible")
	assert.Contains(t, sb, repo_model.IssueCodeSBIngredientMissing)
	assert.Contains(t, sb, repo_model.IssueCodeSBIngredientMismatch)
	assert.Contains(t, sb, repo_model.IssueCodeUSFMInvalid)
	assert.Contains(t, sb, repo_model.IssueCodeRepoNameLanguage)
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
