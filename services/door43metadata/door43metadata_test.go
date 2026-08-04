// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"
	"testing"

	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/dcs"
	api "gitea.dev/modules/structs"
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
