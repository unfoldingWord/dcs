// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/dcs"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/convert"
	"gitea.dev/services/door43healthcheck"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sbScope(bookID string) *dcs.ScopeMap {
	scope := dcs.ScopeMap{bookID: []any{}}
	return &scope
}

func testRepo(id int64) *repo_model.Repository {
	return &repo_model.Repository{
		ID: id,
		Owner: &user_model.User{
			ID:       1,
			Name:     "testowner",
			FullName: "Test Owner",
		},
	}
}

func TestGetDoor43MetadataFromSBMetadata_ParascripturalMissingLocalizedName(t *testing.T) {
	dm := &repo_model.Door43Metadata{}
	repo := testRepo(123)
	sb := &dcs.SBMetadata100{
		Meta: &dcs.SB100Meta{Version: "1.0.0"},
		Identification: &dcs.SB100Identification{
			Name:         dcs.LocalizedText{"en": "English TN"},
			Abbreviation: dcs.LocalizedText{"en": "tn"},
		},
		Languages: []*dcs.SB100Language{{
			Tag:  "en",
			Name: dcs.LocalizedText{"en": "English"},
		}},
		Type: &dcs.SB100Type{
			FlavorType: dcs.SB100FlavorType{
				Name:   "parascriptural",
				Flavor: dcs.SB100Flavor{Name: "x-bcvnotes"},
			},
		},
		LocalizedNames: map[string]*dcs.SB100LocalizedName{},
		Ingredients: map[string]*dcs.SB100Ingredient{
			"ingredients/gen.tsv": {Scope: sbScope("GEN")},
		},
	}

	ctx := context.Background()
	require.NoError(t, GetDoor43MetadataFromSBMetadata(ctx, nil, dm, sb, repo, nil))
	assert.Equal(t, int64(123), dm.RepoID)
	assert.Equal(t, "sb", dm.MetadataType)
	assert.Equal(t, "TSV Translation Notes", dm.Subject)
	assert.Equal(t, "tsv7", dm.ContentFormat)
	require.Len(t, dm.Ingredients, 1)
	assert.Equal(t, "gen", dm.Ingredients[0].Identifier)
	assert.Equal(t, "./ingredients/gen.tsv", dm.Ingredients[0].Path)
	assert.Equal(t, "Genesis", dm.Ingredients[0].Title)
}

func TestGetDoor43MetadataFromSBMetadata_ScriptureSkipsInvalidIngredients(t *testing.T) {
	dm := &repo_model.Door43Metadata{}
	repo := testRepo(456)
	sb := &dcs.SBMetadata100{
		Meta: &dcs.SB100Meta{Version: "1.0.0"},
		Identification: &dcs.SB100Identification{
			Name:         dcs.LocalizedText{"en": "English Bible"},
			Abbreviation: dcs.LocalizedText{"en": "ult"},
		},
		Languages: []*dcs.SB100Language{
			nil,
			{Tag: "en", Name: dcs.LocalizedText{"en": "English"}},
		},
		Type: &dcs.SB100Type{
			FlavorType: dcs.SB100FlavorType{
				Name:   "scripture",
				Flavor: dcs.SB100Flavor{Name: "textTranslation"},
			},
		},
		Ingredients: map[string]*dcs.SB100Ingredient{
			"ingredients/gen.md":      {Scope: sbScope("GEN")},
			"ingredients/empty.md":    nil,
			"ingredients/noscope.txt": {},
		},
	}

	ctx := context.Background()
	require.NoError(t, GetDoor43MetadataFromSBMetadata(ctx, nil, dm, sb, repo, nil))
	assert.Equal(t, "Bible", dm.Subject)
	assert.Equal(t, "md", dm.ContentFormat)
	require.Len(t, dm.Ingredients, 1)
	assert.Equal(t, "gen", dm.Ingredients[0].Identifier)
	assert.Equal(t, "Genesis", dm.Ingredients[0].Title)
	assert.Equal(t, "./ingredients/gen.md", dm.Ingredients[0].Path)
}

// TestProcessDoor43MetadataForRepo_RepoDMAfterCreate replays the notifier
// sequence that runs when a public non-empty repo is created: the ref
// processing memoizes LoadLatestDMs (LatestDMsLoaded=true, RepoDM synthesized),
// then handleRepoDM reassigns RepoDM from the stage DMs. With no valid DM rows
// this leaves RepoDM nil while LatestDMsLoaded stays true, so the actions
// notifier's convert.ToRepo -> ToRepoDCS panics on dm.Title.
func TestProcessDoor43MetadataForRepo_RepoDMAfterCreate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.False(t, repo.IsEmpty)
	require.False(t, repo.IsPrivate)
	require.False(t, repo.LatestDMsLoaded)

	require.NoError(t, ProcessDoor43MetadataForRepo(ctx, repo, repo.DefaultBranch))

	t.Logf("after ProcessDoor43MetadataForRepo: LatestDMsLoaded=%v RepoDM=%+v", repo.LatestDMsLoaded, repo.RepoDM)
	assert.True(t, repo.LatestDMsLoaded, "LatestDMsLoaded should be memoized by the ref processing")
	assert.NotNil(t, repo.RepoDM, "RepoDM must never be nil after processing (ToRepoDCS dereferences it)")
	assert.NotPanics(t, func() {
		convert.ToRepoDCS(ctx, repo, &api.Repository{})
	}, "ToRepoDCS must not panic right after repo creation processing")
}

func TestProcessDoor43MetadataForRepo_DefaultBranchRecheck(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()

	const defaultBranch, userBranch = "dcs-recheck-default", "dcs-recheck-user"
	require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(ctx, &repo_model.Repository{ID: 1, DefaultBranch: defaultBranch}, "default_branch"))
	loadRepo := func(t *testing.T) *repo_model.Repository {
		t.Helper()
		repo, err := repo_model.GetRepositoryByID(ctx, 1)
		require.NoError(t, err)
		return repo
	}
	checkTime := func(t *testing.T, ref string) timeutil.TimeStamp {
		t.Helper()
		dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, 1, ref)
		require.NoError(t, err)
		return dm.HealthcheckTimeUnix
	}
	resetDefaultCheckTime := func(t *testing.T) { // a re-check overwrites it with the current time
		t.Helper()
		dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, 1, defaultBranch)
		require.NoError(t, err)
		dm.HealthcheckTimeUnix = 1
		require.NoError(t, repo_model.UpdateDoor43MetadataCols(ctx, dm, "healthcheck_time_unix"))
	}

	repoPath := gitrepo.RepoLocalPath(loadRepo(t).CodeStorageRepo())
	commitFileOnBranch(t, repoPath, defaultBranch, "manifest.yaml", validRCManifest)
	commitFileOnBranch(t, repoPath, userBranch, "manifest.yaml", validRCManifest)

	require.NoError(t, processDoor43MetadataForRepo(ctx, loadRepo(t), defaultBranch, false))
	defaultDM, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, 1, defaultBranch)
	require.NoError(t, err)
	require.Equal(t, door43metadata.StageLatest, defaultDM.Stage)

	resetDefaultCheckTime(t)
	require.NoError(t, processDoor43MetadataForRepo(ctx, loadRepo(t), userBranch, false))
	assert.EqualValues(t, 1, checkTime(t, defaultBranch), "pushing another branch must not re-check the default branch")
	assert.Greater(t, checkTime(t, userBranch), timeutil.TimeStamp(1), "the pushed branch itself is checked")

	require.NoError(t, processDoor43MetadataForRepo(ctx, loadRepo(t), defaultBranch, false))
	assert.Greater(t, checkTime(t, defaultBranch), timeutil.TimeStamp(1), "pushing the default branch checks it in its own ref pass")

	resetDefaultCheckTime(t)
	require.NoError(t, processDoor43MetadataForRepo(ctx, loadRepo(t), "v1.1", false))
	assert.Greater(t, checkTime(t, defaultBranch), timeutil.TimeStamp(1), "a release can change the default branch's release advice")

	resetDefaultCheckTime(t)
	rel := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{RepoID: 1, LowerTagName: "v1.1"})
	_, err = db.DeleteByID[repo_model.Release](ctx, rel.ID)
	require.NoError(t, err)
	NewNotifier().DeleteRelease(ctx, nil, rel)
	assert.Greater(t, checkTime(t, defaultBranch), timeutil.TimeStamp(1), "deleting a release can change the default branch's release advice")
}

func TestGetBookAlignmentCount(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()

	repo, err := repo_model.GetRepositoryByID(ctx, 1)
	require.NoError(t, err)
	commitFileOnBranch(t, gitrepo.RepoLocalPath(repo.CodeStorageRepo()), "dcs-alignments", "01-GEN.usfm", `\id GEN
\c 1
\v 1 \zaln-s |x-strong="H7225"\*\w In|x-occurrence="1"\w*\zaln-e\*
\v 2 \zaln-s |x-strong="a"\*\zaln-s |x-strong="b"\*\w the\w*\zaln-e\*\zaln-e\*
`)
	gitRepo, err := git.OpenRepository(ctx, repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	commit, err := gitRepo.GetBranchCommit(ctx, "dcs-alignments")
	require.NoError(t, err)

	count, err := GetBookAlignmentCount(ctx, gitRepo, "01-GEN.usfm", commit)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestGetDoor43MetadataFromSBMetadata_AllowsSparseMetadata(t *testing.T) {
	dm := &repo_model.Door43Metadata{}
	repo := testRepo(789)
	sb := &dcs.SBMetadata100{}

	ctx := context.Background()
	require.NoError(t, GetDoor43MetadataFromSBMetadata(ctx, nil, dm, sb, repo, nil))
	assert.Equal(t, int64(789), dm.RepoID)
	assert.Equal(t, "sb", dm.MetadataType)
	assert.Equal(t, "Unknown", dm.Subject)
	assert.Empty(t, dm.FlavorType)
	assert.Empty(t, dm.Flavor)
}

// Regression test: an entry with a schema-invalid metadata file and a pipeline-style
// pre-set repo (owner not loaded) must not panic RunHealthcheck, and must report only
// the invalid-metadata finding — deeper checks would run on backfilled, not extracted,
// fields (crash seen migrating unfoldingWord/en_tn v1, whose 2017 manifest has the
// pre-RC0.2 subject "Translator Notes").
func TestRunHealthcheckInvalidMetadataOwnerNotLoaded(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo, err := repo_model.GetRepositoryByID(t.Context(), 1)
	require.NoError(t, err)
	require.Nil(t, repo.Owner, "test requires a repo whose owner is not yet loaded")

	dm := &repo_model.Door43Metadata{
		RepoID: repo.ID, Ref: "v1", RefType: "tag",
		CommitSHA: "0000000000000000000000000000000000000051",
		Stage:     door43metadata.StageOther,
		Language:  "en", Subject: "TSV Translation Notes", MetadataType: "rc",
		ValidationError: &jsonschema.ValidationError{Message: "value must be one of ..."},
	}
	require.NoError(t, repo_model.InsertDoor43Metadata(t.Context(), dm))
	dm.Repo = repo // as processDoor43MetadataForRepoRef does: repo set, owner not loaded

	hgi := door43healthcheck.RunHealthcheck(t.Context(), dm)
	require.NotNil(t, hgi)
	assert.Equal(t, repo_model.SeverityLevelError, hgi.OverallSeverityLevel)
	require.Len(t, hgi.Issues[repo_model.IssueCodeMetadataInvalid], 1)
	total := 0
	for _, issues := range hgi.Issues {
		total += len(issues)
	}
	assert.Equal(t, 1, total, "only the invalid-metadata finding should be reported for a schema-invalid entry")
}
