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
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/structs"
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
			// The latest entry carries the media flags of ALL the repo's releases
			// of its stage (v0's audio and stream plus its own PDF), as maintained
			// by repo_model.AggregateMediaFlagsForRepo
			HasPDF: true, HasAudio: true, HasStream: true,
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
	// The has_* counts are repo counts from the latest entries, whose flags
	// aggregate every release of the stage (v0's audio and stream count via v1)
	assert.EqualValues(t, 1, stats.HasPDF)
	assert.EqualValues(t, 1, stats.HasAudio)
	assert.EqualValues(t, 0, stats.HasVideo)
	assert.EqualValues(t, 1, stats.HasStream)
	assert.EqualValues(t, 0, stats.HasOther)
	// repo1 has media in both its releases but still counts once
	assert.EqualValues(t, 1, stats.HasAttachment)

	// includeHistory is ignored: entry counts stay repo counts over the latest
	// entries, and the media counts already considered all releases above
	stats, err = GetCatalogStats(t.Context(), &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd, IncludeHistory: true})
	require.NoError(t, err)
	assert.EqualValues(t, 2, stats.EntryCount)
	assert.EqualValues(t, 2, stats.LangCount)
	assert.EqualValues(t, 2, stats.RepoCount)
	assert.EqualValues(t, 1, stats.RcCount)
	assert.EqualValues(t, 1, stats.HasAudio)
	assert.EqualValues(t, 1, stats.HasStream)
	assert.EqualValues(t, 1, stats.HasAttachment)

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
	// Audio only exists in repo1's older v0 release, but the aggregated flags on
	// its latest entry make the hasAudio filter match the repo
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
	assert.EqualValues(t, 1, ext.IsHealthyCount)
	assert.EqualValues(t, 1, ext.IsHealthyWithoutWarningsCount)
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
	assert.EqualValues(t, 0, ext.IsHealthyCount)
	assert.EqualValues(t, 0, ext.IsHealthyWithoutWarningsCount)
	assert.Equal(t, map[string]int64{}, ext.Subjects)
	assert.Equal(t, map[string]int64{}, ext.Owners)
}

// TestSearchCatalogIsHealthyFilter checks the is_healthy / is_healthy_without_warnings
// filters: warnings count as healthy for is_healthy but not for the stricter filter, and
// never-checked entries (NULL or 0 severity) are not healthy under either.
func TestSearchCatalogIsHealthyFilter(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	dms := []*repo_model.Door43Metadata{
		{
			RepoID: 1, ReleaseID: 3001, Ref: "v1", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000021",
			Stage: door43metadata.StageProd, IsLatestForStage: true,
			Language: "en", Subject: "Open Bible Stories", MetadataType: "rc",
			HealthcheckSeverity: repo_model.SeverityLevelWarning,
			ReleaseDateUnix:     timeutil.TimeStamp(1000),
		},
		{
			RepoID: 4, ReleaseID: 3002, Ref: "v1", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000022",
			Stage: door43metadata.StageProd, IsLatestForStage: true,
			Language: "fr", Subject: "Bible", MetadataType: "sb",
			HealthcheckSeverity: repo_model.SeverityLevelSuccess,
			ReleaseDateUnix:     timeutil.TimeStamp(2000),
		},
		{
			// never checked: severity stays 0/NULL
			RepoID: 10, ReleaseID: 3003, Ref: "v1", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000023",
			Stage: door43metadata.StageProd, IsLatestForStage: true,
			Language: "es", Subject: "Bible", MetadataType: "tc",
			ReleaseDateUnix: timeutil.TimeStamp(3000),
		},
	}
	for _, dm := range dms {
		_, err := db.GetEngine(t.Context()).Insert(dm)
		require.NoError(t, err)
	}

	search := func(opts *door43metadata.SearchCatalogOptions) []int64 {
		opts.Stage = door43metadata.StageProd
		results, _, err := SearchCatalog(t.Context(), opts)
		require.NoError(t, err)
		repoIDs := make([]int64, 0, len(results))
		for _, dm := range results {
			repoIDs = append(repoIDs, dm.RepoID)
		}
		return repoIDs
	}

	// warnings are healthy by default
	assert.ElementsMatch(t, []int64{1, 4}, search(&door43metadata.SearchCatalogOptions{IsHealthy: optional.Some(true)}))
	// not healthy = errors and never-checked entries
	assert.ElementsMatch(t, []int64{10}, search(&door43metadata.SearchCatalogOptions{IsHealthy: optional.Some(false)}))
	// the stricter filter drops warnings too
	assert.ElementsMatch(t, []int64{4}, search(&door43metadata.SearchCatalogOptions{IsHealthyWithoutWarnings: optional.Some(true)}))
	assert.ElementsMatch(t, []int64{1, 10}, search(&door43metadata.SearchCatalogOptions{IsHealthyWithoutWarnings: optional.Some(false)}))
}

// TestAggregateMediaFlagsForRepo checks that the latest prod entry of a repo gets the
// media flags of all the repo's prod releases, that flags not backed by any release
// are cleared, and that non-latest entries keep the flags of their own release.
func TestAggregateMediaFlagsForRepo(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	v0 := &repo_model.Door43Metadata{
		RepoID: 1, ReleaseID: 3000, Ref: "media-v0", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000020",
		Stage: door43metadata.StageProd, IsLatestForStage: false,
		Language: "en", Subject: "Open Bible Stories", MetadataType: "rc",
		HasAudio: true, HasStream: true,
		ReleaseDateUnix: timeutil.TimeStamp(500),
	}
	// The latest entry starts with a stale PDF flag not backed by any attachment
	v1 := &repo_model.Door43Metadata{
		RepoID: 1, ReleaseID: 3001, Ref: "media-v1", RefType: "tag", CommitSHA: "0000000000000000000000000000000000000021",
		Stage: door43metadata.StageProd, IsLatestForStage: true,
		Language: "en", Subject: "Open Bible Stories", MetadataType: "rc",
		HasPDF:          true,
		ReleaseDateUnix: timeutil.TimeStamp(1000),
	}
	for _, dm := range []*repo_model.Door43Metadata{v0, v1} {
		_, err := db.GetEngine(t.Context()).Insert(dm)
		require.NoError(t, err)
	}

	// v1 aggregates v0's audio and stream; its stale PDF flag is cleared since
	// release 3001 has no PDF attachment backing it
	require.NoError(t, repo_model.AggregateMediaFlagsForRepo(t.Context(), 1))
	latest, err := repo_model.GetDoor43MetadataByID(t.Context(), v1.ID)
	require.NoError(t, err)
	assert.True(t, latest.HasAudio)
	assert.True(t, latest.HasStream)
	assert.False(t, latest.HasPDF)
	assert.False(t, latest.HasVideo)
	assert.False(t, latest.HasOther)

	// The non-latest entry keeps the flags of its own release
	old, err := repo_model.GetDoor43MetadataByID(t.Context(), v0.ID)
	require.NoError(t, err)
	assert.True(t, old.HasAudio)
	assert.True(t, old.HasStream)
	assert.False(t, old.HasPDF)

	// Deleting the release that provided the media clears the aggregate
	require.NoError(t, repo_model.DeleteDoor43Metadata(t.Context(), v0))
	require.NoError(t, repo_model.AggregateMediaFlagsForRepo(t.Context(), 1))
	latest, err = repo_model.GetDoor43MetadataByID(t.Context(), v1.ID)
	require.NoError(t, err)
	assert.False(t, latest.HasAudio)
	assert.False(t, latest.HasStream)
	assert.False(t, latest.HasPDF)
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
	expectedReleaseCount, err := results[0].ReleaseCount(t.Context())
	require.NoError(t, err)
	require.NoError(t, results.LoadReleaseCounts(t.Context()))
	assert.Equal(t, expectedReleaseCount, results[0].ReleaseCountValue)

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

// TestSearchCatalogKeywordUnderscoreIsLiteral exercises GetMetadataCond's LIKE
// conditions against a real database (not just the generated SQL text), so a query
// that's syntactically valid Go but gets mis-parsed by a specific SQL dialect (as
// happened with an early "ESCAPE '\'" version of this condition on MySQL, which
// SQLite's different string-literal rules didn't surface) fails a test instead of
// only showing up against a live server.
//
// It also checks the actual behavior the escaping is for: a keyword search for
// "jup_mat" must match a title containing that literal substring, but not one that
// only matches if "_" is read as the SQL LIKE wildcard for "any single character".
func TestSearchCatalogKeywordUnderscoreIsLiteral(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	literalMatch := &repo_model.Door43Metadata{
		RepoID: 1, Ref: "master", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000001",
		Stage: door43metadata.StageProd, IsLatestForStage: true,
		Language: "en", Subject: "Open Bible Stories", Title: "jup_mat resource",
	}
	wildcardOnlyMatch := &repo_model.Door43Metadata{
		RepoID: 33, Ref: "master", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000002",
		Stage: door43metadata.StageProd, IsLatestForStage: true,
		Language: "en", Subject: "Open Bible Stories", Title: "jupXmat resource",
	}
	for _, dm := range []*repo_model.Door43Metadata{literalMatch, wildcardOnlyMatch} {
		_, err := db.GetEngine(t.Context()).Insert(dm)
		require.NoError(t, err)
	}

	opts := &door43metadata.SearchCatalogOptions{
		Stage:    door43metadata.StageProd,
		Keywords: []string{"jup_mat"},
	}
	results, count, err := SearchCatalog(t.Context(), opts)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.Len(t, results, 1)
	assert.Equal(t, literalMatch.RepoID, results[0].RepoID)
}

// TestSearchCatalogForBookPackage checks which entry each repo of a book package
// contributes for a requested ref. repo1 (user2) is the requested repo; its partners
// are user2's other public repos: utf8 (33) with master and releases, repo-release (57)
// with only a "main" default branch, commits_search_test (36) with only a release, and
// git_hooks_test (37) with a "topic" branch besides master.
func TestSearchCatalogForBookPackage(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	const (
		requestedRepo = int64(1)
		withBranches  = int64(33)
		mainOnly      = int64(57)
		releaseOnly   = int64(36)
		withTopic     = int64(37)
	)
	type row struct {
		repoID  int64
		ref     string
		refType string
		stage   door43metadata.Stage
		latest  bool
		release timeutil.TimeStamp
	}
	rows := []row{
		{requestedRepo, "master", "branch", door43metadata.StageLatest, true, 1000},
		{requestedRepo, "topic", "branch", door43metadata.StageOther, false, 1000},
		{requestedRepo, "v1", "tag", door43metadata.StageProd, false, 1000},
		{requestedRepo, "v2", "tag", door43metadata.StageProd, true, 5000},

		{withBranches, "master", "branch", door43metadata.StageLatest, true, 6000},
		{withBranches, "v1", "tag", door43metadata.StageProd, false, 900},
		{withBranches, "v1.1", "tag", door43metadata.StagePreProd, false, 1010},
		{withBranches, "v2", "tag", door43metadata.StageProd, true, 4000},

		{mainOnly, "main", "branch", door43metadata.StageLatest, true, 999},

		{releaseOnly, "v1", "tag", door43metadata.StageProd, true, 900},

		{withTopic, "master", "branch", door43metadata.StageLatest, true, 6000},
		{withTopic, "topic", "branch", door43metadata.StageOther, false, 7000},
	}
	for i, r := range rows {
		_, err := db.GetEngine(t.Context()).Insert(&repo_model.Door43Metadata{
			RepoID: r.repoID, Ref: r.ref, RefType: r.refType, CommitSHA: fmt.Sprintf("%040d", i+1),
			Stage: r.stage, IsLatestForStage: r.latest, ReleaseDateUnix: r.release,
			Language: "en", Subject: "Translation Words", MetadataType: "rc",
		})
		require.NoError(t, err)
	}

	requested := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: requestedRepo})
	search := func(ref string, opts *door43metadata.SearchCatalogOptions) map[int64]string {
		dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(t.Context(), requestedRepo, ref)
		require.NoError(t, err)
		dm.Repo = requested
		if opts == nil {
			opts = &door43metadata.SearchCatalogOptions{}
		}
		opts.IncludeHistory = true
		results, _, err := SearchCatalogForBookPackage(t.Context(), dm, opts)
		require.NoError(t, err)
		refs := make(map[int64]string, len(results))
		for _, result := range results {
			refs[result.RepoID] = result.Ref
		}
		return refs
	}

	// A default-branch request pairs with each partner's same-named branch, else its
	// default branch, and only falls back to the closest release when it has no branch
	// entry at all; utf8's v1 is closer in time than its master and must not win.
	assert.Equal(t, map[int64]string{
		requestedRepo: "master", withBranches: "master", mainOnly: "main", releaseOnly: "v1", withTopic: "master",
	}, search("master", nil))

	// A feature-branch request pairs with the same-named branch where it exists.
	assert.Equal(t, map[int64]string{
		requestedRepo: "topic", withBranches: "master", mainOnly: "main", releaseOnly: "v1", withTopic: "topic",
	}, search("topic", nil))

	// A release pairs with each partner's release closest in time (same stage or better,
	// so the closer pre-release is skipped), and with the default branch of a partner
	// that has no release at all.
	assert.Equal(t, map[int64]string{
		requestedRepo: "v1", withBranches: "v1", mainOnly: "main", releaseOnly: "v1", withTopic: "master",
	}, search("v1", nil))
	assert.Equal(t, map[int64]string{
		requestedRepo: "v2", withBranches: "v2", mainOnly: "main", releaseOnly: "v1", withTopic: "master",
	}, search("v2", nil))

	// An explicit stage filter still limits the candidates: branches are no longer
	// eligible, so a branch request falls back to the closest production release.
	assert.Equal(t, map[int64]string{
		requestedRepo: "v1", withBranches: "v1", releaseOnly: "v1",
	}, search("master", &door43metadata.SearchCatalogOptions{Stage: door43metadata.StageProd}))
}

// TestSearchCatalogForBookPackageRelations checks the entries a book package takes from the
// requested entry's manifest relations. repo1 (user2) relates to en/ult, which user2 has,
// and to en/ust?v=1 and en/ta, which only unfoldingWord has; ta is outside the requested
// subjects. Its relations pin hbo/uhb but not el-x-koine/ugnt.
func TestSearchCatalogForBookPackageRelations(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	uw := &user_model.User{Name: "unfoldingWord", LowerName: "unfoldingword", Email: "uw@example.com", Type: user_model.UserTypeOrganization}
	require.NoError(t, db.Insert(ctx, uw))
	newRepo := func(owner *user_model.User, name string) int64 {
		r := &repo_model.Repository{OwnerID: owner.ID, OwnerName: owner.Name, Name: name, LowerName: name}
		require.NoError(t, db.Insert(ctx, r))
		return r.ID
	}
	const requestedRepo = int64(1)
	ownULT := newRepo(user2, "en_ult")
	uwULT := newRepo(uw, "en_ult")
	uwUST := newRepo(uw, "en_ust")
	uwTA := newRepo(uw, "en_ta")
	uhb := newRepo(uw, "hbo_uhb")
	ugnt := newRepo(uw, "el-x-koine_ugnt")
	// en/glt?v=3: unfoldingWord has en_glt but not v3, Door43-Catalog has v3
	d43 := &user_model.User{Name: "Door43-Catalog", LowerName: "door43-catalog", Email: "d43@example.com", Type: user_model.UserTypeOrganization}
	require.NoError(t, db.Insert(ctx, d43))
	uwGLT := newRepo(uw, "en_glt")
	d43GLT := newRepo(d43, "en_glt")

	relation := func(lang, id, version string) *structs.Relation {
		return &structs.Relation{Language: lang, Identifier: id, Version: version}
	}
	type row struct {
		repoID   int64
		ref      string
		stage    door43metadata.Stage
		release  timeutil.TimeStamp
		language string
		subject  string
	}
	rows := []row{
		{requestedRepo, "master", door43metadata.StageLatest, 1000, "en", "TSV Study Notes"},
		{ownULT, "master", door43metadata.StageLatest, 1000, "en", "Aligned Bible"},
		{uwULT, "master", door43metadata.StageLatest, 1000, "en", "Aligned Bible"},
		{uwUST, "master", door43metadata.StageLatest, 1000, "en", "Aligned Bible"},
		{uwUST, "v1", door43metadata.StageProd, 100, "en", "Aligned Bible"},
		{uwUST, "v2", door43metadata.StageProd, 990, "en", "Aligned Bible"},
		{uwTA, "master", door43metadata.StageLatest, 1000, "en", "Translation Academy"},
		{uhb, "v2.1.17", door43metadata.StageProd, 100, "hbo", "Hebrew Old Testament"},
		{uhb, "v2.1.30", door43metadata.StageProd, 990, "hbo", "Hebrew Old Testament"},
		{ugnt, "v0.18", door43metadata.StageProd, 100, "el-x-koine", "Greek New Testament"},
		{ugnt, "v0.30", door43metadata.StageProd, 990, "el-x-koine", "Greek New Testament"},
		{uwGLT, "master", door43metadata.StageLatest, 1000, "en", "Aligned Bible"},
		{d43GLT, "v3", door43metadata.StageProd, 100, "en", "Aligned Bible"},
	}
	for i, r := range rows {
		dm := &repo_model.Door43Metadata{
			RepoID: r.repoID, Ref: r.ref, RefType: "tag", CommitSHA: fmt.Sprintf("%040d", i+1),
			Stage: r.stage, IsLatestForStage: true, ReleaseDateUnix: r.release,
			Language: r.language, Subject: r.subject, MetadataType: "rc",
		}
		if r.stage == door43metadata.StageLatest {
			dm.RefType = "branch"
		}
		if r.repoID == requestedRepo {
			dm.Relations = []*structs.Relation{relation("en", "ult", ""), relation("en", "ust", "1"), relation("en", "ta", ""), relation("hbo", "uhb", "2.1.17"), relation("el-x-koine", "ugnt", ""), relation("en", "glt", "3")}
		}
		_, err := db.GetEngine(ctx).Insert(dm)
		require.NoError(t, err)
	}

	dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, requestedRepo, "master")
	require.NoError(t, err)
	dm.Repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: requestedRepo})
	refs := func(dms repo_model.Door43MetadataList) map[int64]string {
		m := make(map[int64]string, len(dms))
		for _, entry := range dms {
			m[entry.RepoID] = entry.Ref
		}
		return m
	}

	// user2's own en_ult is kept over unfoldingWord's; unfoldingWord's en_ust comes at the
	// pinned v1 rather than its master; en_ta is filtered out by the subjects like any entry;
	// en_glt comes from Door43-Catalog at the pinned v3, not from unfoldingWord's master.
	results, count, err := SearchCatalogForBookPackage(ctx, dm, &door43metadata.SearchCatalogOptions{
		Subjects:       []string{"TSV Study Notes", "Aligned Bible"},
		IncludeHistory: true,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 4, count)
	assert.Equal(t, map[int64]string{requestedRepo: "master", ownULT: "master", uwUST: "v1", d43GLT: "v3"}, refs(results))

	// The pinned Hebrew release wins over the one closer in time; the unpinned Greek
	// Bible is still the release closest in time.
	results, _, err = GetOrigLanguageBibles(ctx, dm, nil)
	require.NoError(t, err)
	assert.Equal(t, map[int64]string{uhb: "v2.1.17", ugnt: "v0.30"}, refs(results))
}
