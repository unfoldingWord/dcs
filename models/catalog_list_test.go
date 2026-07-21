// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearchDoor43MetadataFieldContentFlags reproduces the has_* / includeHistory
// semantics for the /catalog/list/* endpoints: repo1 has a v9 release (latest
// prod) with only a PDF, and an older v8 release with audio and a YouTube link.
func TestSearchDoor43MetadataFieldContentFlags(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	dms := []*repo_model.Door43Metadata{
		{
			RepoID: 1, ReleaseID: 1001, Ref: "v8", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000008",
			Stage: door43metadata.StageProd, IsLatestForStage: false,
			Language: "en", Subject: "Open Bible Stories",
			HasAudio: true, HasStream: true,
			ReleaseDateUnix: timeutil.TimeStamp(1000),
		},
		{
			RepoID: 1, ReleaseID: 1002, Ref: "v9", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000009",
			Stage: door43metadata.StageProd, IsLatestForStage: true,
			Language: "en", Subject: "Open Bible Stories",
			HasPDF:          true,
			ReleaseDateUnix: timeutil.TimeStamp(2000),
		},
	}
	for _, dm := range dms {
		_, err := db.GetEngine(t.Context()).Insert(dm)
		require.NoError(t, err)
	}

	newOpts := func() *door43metadata.SearchCatalogOptions {
		return &door43metadata.SearchCatalogOptions{
			Stage:     door43metadata.StageProd,
			Languages: []string{"en"},
			Subjects:  []string{"Open Bible Stories"},
		}
	}

	// hasAudio=1 without includeHistory: v9 (latest prod) has no audio -> no languages
	opts := newOpts()
	opts.HasAudio = optional.Some(true)
	langs, err := SearchDoor43MetadataField(t.Context(), opts, "language")
	require.NoError(t, err)
	assert.Empty(t, langs)

	// hasAudio=1 with includeHistory: v8 has audio -> "en"
	opts.IncludeHistory = true
	langs, err = SearchDoor43MetadataField(t.Context(), opts, "language")
	require.NoError(t, err)
	assert.Equal(t, []string{"en"}, langs)

	// hasPdf=1 without includeHistory: v9 has a PDF -> "en"
	opts = newOpts()
	opts.HasPDF = optional.Some(true)
	langs, err = SearchDoor43MetadataField(t.Context(), opts, "language")
	require.NoError(t, err)
	assert.Equal(t, []string{"en"}, langs)

	// hasStream=1 without includeHistory: only v8 has a stream -> no languages
	opts = newOpts()
	opts.HasStream = optional.Some(true)
	langs, err = SearchDoor43MetadataField(t.Context(), opts, "language")
	require.NoError(t, err)
	assert.Empty(t, langs)

	// hasAttachment=1 with includeHistory: both releases match but results are distinct -> just one "en"
	opts = newOpts()
	opts.HasAttachment = optional.Some(true)
	opts.IncludeHistory = true
	langs, err = SearchDoor43MetadataField(t.Context(), opts, "language")
	require.NoError(t, err)
	assert.Equal(t, []string{"en"}, langs)

	// hasAttachment=0: no entry is without attachments -> no languages
	opts = newOpts()
	opts.HasAttachment = optional.Some(false)
	opts.IncludeHistory = true
	langs, err = SearchDoor43MetadataField(t.Context(), opts, "language")
	require.NoError(t, err)
	assert.Empty(t, langs)

	// owners list uses the user table join; repo1 belongs to user2
	opts = newOpts()
	opts.HasAttachment = optional.Some(true)
	opts.IncludeHistory = true
	owners, err := SearchDoor43MetadataField(t.Context(), opts, "`user`.lower_name")
	require.NoError(t, err)
	assert.Equal(t, []string{"user2"}, owners)
}

// TestGetCatalogStats checks the aggregate counts and unique-value lists of the
// stats endpoints: repo1 (user2) has a latest prod release with a PDF and a
// healthcheck, plus an older release with audio and a stream; repo4 (user5) has
// a latest prod release with no attachments.
func TestGetCatalogStats(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	dms := []*repo_model.Door43Metadata{
		{
			RepoID: 1, ReleaseID: 2000, Ref: "v0", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000010",
			Stage: door43metadata.StageProd, IsLatestForStage: false,
			Language: "en", LanguageDirection: "ltr", Subject: "Open Bible Stories",
			MetadataType: "rc", FlavorType: "gloss", Flavor: "textStories",
			HasAudio: true, HasStream: true,
			ReleaseDateUnix: timeutil.TimeStamp(500),
		},
		{
			RepoID: 1, ReleaseID: 2001, Ref: "v1", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000011",
			Stage: door43metadata.StageProd, IsLatestForStage: true,
			Language: "en", LanguageDirection: "ltr", Subject: "Open Bible Stories",
			MetadataType: "rc", FlavorType: "gloss", Flavor: "textStories",
			HasPDF:              true,
			HealthcheckSeverity: repo_model.SeverityLevelSuccess,
			ReleaseDateUnix:     timeutil.TimeStamp(1000),
		},
		{
			RepoID: 4, ReleaseID: 2002, Ref: "v1", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000012",
			Stage: door43metadata.StageProd, IsLatestForStage: true,
			Language: "ar", LanguageDirection: "rtl", Subject: "Bible",
			MetadataType: "sb", FlavorType: "scripture", Flavor: "textTranslation",
			ReleaseDateUnix: timeutil.TimeStamp(2000),
		},
	}
	for _, dm := range dms {
		_, err := db.GetEngine(t.Context()).Insert(dm)
		require.NoError(t, err)
	}

	// Default (stage prod, no history): the two latest prod entries
	stats, err := GetCatalogStats(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd})
	require.NoError(t, err)
	assert.EqualValues(t, 2, stats.EntryCount)
	assert.EqualValues(t, 2, stats.LangCount)
	assert.EqualValues(t, 1, stats.LangLtrCount)
	assert.EqualValues(t, 1, stats.LangRtlCount)
	assert.EqualValues(t, 2, stats.SubjectCount)
	assert.EqualValues(t, 2, stats.FlavorTypeCount)
	assert.EqualValues(t, 2, stats.FlavorCount)
	assert.EqualValues(t, 2, stats.OwnerCount)
	assert.EqualValues(t, 2, stats.RepoCount)
	assert.EqualValues(t, 0, stats.TsCount)
	assert.EqualValues(t, 0, stats.TcCount)
	assert.EqualValues(t, 1, stats.RcCount)
	assert.EqualValues(t, 1, stats.SbCount)
	assert.EqualValues(t, 1, stats.HasPDF)
	assert.EqualValues(t, 0, stats.HasAudio)
	assert.EqualValues(t, 0, stats.HasVideo)
	assert.EqualValues(t, 0, stats.HasStream)
	assert.EqualValues(t, 0, stats.HasOther)
	assert.EqualValues(t, 1, stats.HasAttachment)

	// includeHistory picks up the older v0 release of repo1 too
	stats, err = GetCatalogStats(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd, IncludeHistory: true})
	require.NoError(t, err)
	assert.EqualValues(t, 3, stats.EntryCount)
	assert.EqualValues(t, 2, stats.LangCount)
	assert.EqualValues(t, 2, stats.RepoCount)
	assert.EqualValues(t, 2, stats.RcCount)
	assert.EqualValues(t, 1, stats.HasAudio)
	assert.EqualValues(t, 1, stats.HasStream)
	// v0 has two content types (audio + stream) but counts once; v1 has a PDF
	assert.EqualValues(t, 2, stats.HasAttachment)

	// Topic filters work like the other catalog searches: repo1 has the
	// "golang" topic in the fixtures, repo4 has no topics
	stats, err = GetCatalogStats(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd, Topics: []string{"golang"}})
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.EntryCount)
	assert.EqualValues(t, 1, stats.HasPDF)
	stats, err = GetCatalogStats(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd, InvertedTopics: []string{"golang"}})
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.EntryCount)
	assert.EqualValues(t, 1, stats.LangRtlCount)

	// The has* filters narrow the stats both ways (true and false)
	stats, err = GetCatalogStats(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd, HasPDF: optional.Some(false)})
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.EntryCount)
	assert.EqualValues(t, 0, stats.HasPDF)
	stats, err = GetCatalogStats(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd, HasAudio: optional.Some(true), IncludeHistory: true})
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.EntryCount)
	assert.EqualValues(t, 1, stats.HasAudio)

	// Date bounds are inclusive on release_date_unix
	stats, err = GetCatalogStats(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd, StartDateUnix: 1500})
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.EntryCount)
	assert.EqualValues(t, 1, stats.LangRtlCount)
	stats, err = GetCatalogStats(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd, EndDateUnix: 1000})
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.EntryCount)
	assert.EqualValues(t, 1, stats.HasPDF)

	// stats-ext adds the healthcheck counts and the entry counts per subject,
	// flavor type, flavor, owner, language and metadata type
	ext, err := GetCatalogStatsExt(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd})
	require.NoError(t, err)
	assert.EqualValues(t, 2, ext.EntryCount)
	assert.EqualValues(t, 1, ext.HealthcheckSuccessCount)
	assert.EqualValues(t, 0, ext.HealthcheckInfoCount)
	assert.EqualValues(t, 0, ext.HealthcheckWarningCount)
	assert.EqualValues(t, 0, ext.HealthcheckErrorCount)
	assert.EqualValues(t, 1, ext.NoHealthcheckCount)
	assert.Equal(t, map[string]int64{"Bible": 1, "Open Bible Stories": 1}, ext.Subjects)
	assert.Equal(t, map[string]int64{"gloss": 1, "scripture": 1}, ext.FlavorTypes)
	assert.Equal(t, map[string]int64{"textStories": 1, "textTranslation": 1}, ext.Flavors)
	assert.Equal(t, map[string]int64{"user2": 1, "user5": 1}, ext.Owners)
	assert.Equal(t, map[string]int64{"ar": 1, "en": 1}, ext.Languages)
	assert.Equal(t, map[string]int64{"rc": 1, "sb": 1}, ext.MetadataTypes)

	// A filter matching nothing returns zero counts and empty (not null) maps
	ext, err = GetCatalogStatsExt(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd, Languages: []string{"zz"}})
	require.NoError(t, err)
	assert.EqualValues(t, 0, ext.EntryCount)
	assert.EqualValues(t, 0, ext.HasAttachment)
	assert.EqualValues(t, 0, ext.NoHealthcheckCount)
	assert.Equal(t, map[string]int64{}, ext.Subjects)
	assert.Equal(t, map[string]int64{}, ext.Owners)
}

// TestSearchCatalogContentFlags checks the full catalog search with the has_*
// filters, including that the flag columns are loaded on the results.
func TestSearchCatalogContentFlags(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	dm := &repo_model.Door43Metadata{
		RepoID: 1, ReleaseID: 1003, Ref: "v1", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000001",
		Stage: door43metadata.StageProd, IsLatestForStage: true,
		Language: "en", Subject: "Open Bible Stories",
		HasAudio: true, HasStream: true,
		ReleaseDateUnix: timeutil.TimeStamp(1000),
	}
	_, err := db.GetEngine(t.Context()).Insert(dm)
	require.NoError(t, err)

	opts := &door43metadata.SearchCatalogOptions{
		Stage:    door43metadata.StageProd,
		HasAudio: optional.Some(true),
	}
	results, count, err := SearchCatalog(t.Context(), opts)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
	require.Len(t, results, 1)
	assert.True(t, results[0].HasAudio)
	assert.True(t, results[0].HasStream)
	assert.False(t, results[0].HasPDF)

	opts.HasAudio = optional.Some(false)
	_, count, err = SearchCatalog(t.Context(), opts)
	require.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

// TestSearchCatalogCounts checks that the unpaginated fast path (no COUNT
// query) reports the same total as the paginated path.
func TestSearchCatalogCounts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for i, flags := range []struct{ audio, pdf bool }{{true, false}, {false, true}, {false, false}} {
		dm := &repo_model.Door43Metadata{
			RepoID: 1, ReleaseID: int64(3000 + i), Ref: fmt.Sprintf("v%d", i+1), RefType: "tag",
			CommitSHA: "0000000000000000000000000000000000003000",
			Stage:     door43metadata.StageProd, IsLatestForStage: i == 2,
			Language: "en", Subject: "Open Bible Stories",
			HasAudio: flags.audio, HasPDF: flags.pdf,
			ReleaseDateUnix: timeutil.TimeStamp(1000 + i),
		}
		_, err := db.GetEngine(t.Context()).Insert(dm)
		require.NoError(t, err)
	}

	newOpts := func() *door43metadata.SearchCatalogOptions {
		return &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd, IncludeHistory: true}
	}

	all, total, err := SearchCatalog(t.Context(), newOpts())
	require.NoError(t, err)
	assert.EqualValues(t, len(all), total)
	assert.EqualValues(t, 3, total)
	for _, dm := range all {
		assert.NotNil(t, dm.Repo, "repo should be batch-loaded")
		assert.NotNil(t, dm.Repo.Owner, "owner should be batch-loaded")
	}

	opts := newOpts()
	opts.ListOptions = db.ListOptions{Page: 1, PageSize: 2}
	page, pagedTotal, err := SearchCatalog(t.Context(), opts)
	require.NoError(t, err)
	assert.Len(t, page, 2)
	assert.Equal(t, total, pagedTotal, "paginated count must match unpaginated total")
}
