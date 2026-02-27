// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/dcs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sbScope(bookID string) *dcs.ScopeMap {
	scope := dcs.ScopeMap{bookID: []any{}}
	return &scope
}

func TestGetDoor43MetadataFromSBMetadata_ParascripturalMissingLocalizedName(ctx context.Context, t *testing.T) {
	dm := &repo_model.Door43Metadata{}
	repo := &repo_model.Repository{ID: 123}
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

	require.NoError(t, GetDoor43MetadataFromSBMetadata(ctx, dm, sb, repo, nil))
	assert.Equal(t, int64(123), dm.RepoID)
	assert.Equal(t, "sb", dm.MetadataType)
	assert.Equal(t, "TSV Translation Notes", dm.Subject)
	assert.Equal(t, "tsv7", dm.ContentFormat)
	require.Len(t, dm.Ingredients, 1)
	assert.Equal(t, "gen", dm.Ingredients[0].Identifier)
	assert.Equal(t, "./ingredients/gen.tsv", dm.Ingredients[0].Path)
	assert.Equal(t, "Genesis", dm.Ingredients[0].Title)
}

func TestGetDoor43MetadataFromSBMetadata_ScriptureSkipsInvalidIngredients(ctx context.Context, t *testing.T) {
	dm := &repo_model.Door43Metadata{}
	repo := &repo_model.Repository{ID: 456}
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

	require.NoError(t, GetDoor43MetadataFromSBMetadata(ctx, dm, sb, repo, nil))
	assert.Equal(t, "Bible", dm.Subject)
	assert.Equal(t, "md", dm.ContentFormat)
	require.Len(t, dm.Ingredients, 1)
	assert.Equal(t, "gen", dm.Ingredients[0].Identifier)
	assert.Equal(t, "Genesis", dm.Ingredients[0].Title)
	assert.Equal(t, "./ingredients/gen.md", dm.Ingredients[0].Path)
}

func TestGetDoor43MetadataFromSBMetadata_AllowsSparseMetadata(ctx context.Context, t *testing.T) {
	dm := &repo_model.Door43Metadata{}
	repo := &repo_model.Repository{ID: 789}
	sb := &dcs.SBMetadata100{}

	require.NoError(t, GetDoor43MetadataFromSBMetadata(ctx, dm, sb, repo, nil))
	assert.Equal(t, int64(789), dm.RepoID)
	assert.Equal(t, "sb", dm.MetadataType)
	assert.Equal(t, "Unknown", dm.Subject)
	assert.Empty(t, dm.FlavorType)
	assert.Empty(t, dm.Flavor)
}
