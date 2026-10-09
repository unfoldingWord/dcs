// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckTitle(t *testing.T) {
	newDM := func(ownerLower, title string) *repo_model.Door43Metadata {
		return &repo_model.Door43Metadata{
			MetadataType: "rc", Subject: "TSV Translation Notes", Ref: "master",
			Title: title,
			Repo: &repo_model.Repository{
				Name: "en_tn", OwnerName: ownerLower,
				Owner: &user_model.User{Name: ownerLower, LowerName: ownerLower},
			},
		}
	}

	// exempt orgs legitimately keep "unfoldingWord" in their titles (case-insensitive)
	for _, owner := range []string{"unfoldingword", "door43-catalog", "uw"} {
		assert.Empty(t, checkTitle(t.Context(), newDM(owner, "unfoldingWord Translation Notes")), owner)
	}

	// other owners get a Warning for an untranslated or empty title
	issues := checkTitle(t.Context(), newDM("someuser", "unfoldingWord Translation Notes"))
	require.Len(t, issues, 1)
	assert.Equal(t, repo_model.IssueCodeTitle, issues[0].IssueCode)
	assert.Equal(t, repo_model.SeverityLevelWarning, issues[0].SeverityLevel)

	// a properly changed title passes
	assert.Empty(t, checkTitle(t.Context(), newDM("someuser", "Notas de Traducción")))
}

func TestCheckPublisher(t *testing.T) {
	newDM := func(ownerLower, publisher string) *repo_model.Door43Metadata {
		return &repo_model.Door43Metadata{
			MetadataType: "rc", Subject: "TSV Translation Notes", Ref: "master",
			Publisher: publisher,
			Repo: &repo_model.Repository{
				Name: "en_tn", OwnerName: ownerLower,
				Owner: &user_model.User{Name: ownerLower, LowerName: ownerLower},
			},
		}
	}

	// the same orgs exempt from the title check keep their unfoldingWord publisher
	for _, owner := range []string{"unfoldingword", "door43-catalog", "uw"} {
		assert.Empty(t, checkPublisher(t.Context(), newDM(owner, "unfoldingWord")), owner)
	}

	// other owners get a Warning for an unchanged or empty publisher
	issues := checkPublisher(t.Context(), newDM("someuser", "unfoldingWord"))
	require.Len(t, issues, 1)
	assert.Equal(t, repo_model.IssueCodePublisher, issues[0].IssueCode)
	assert.Equal(t, repo_model.SeverityLevelWarning, issues[0].SeverityLevel)

	// a properly changed publisher passes
	assert.Empty(t, checkPublisher(t.Context(), newDM("someuser", "Iglesia Ejemplo")))
}

func TestCheckIngredientsSkipsOBSStories(t *testing.T) {
	dm := &repo_model.Door43Metadata{
		MetadataType: "rc", Subject: "Open Bible Stories", Language: "fr", Ref: "master",
		Repo: &repo_model.Repository{
			Name: "fr_obs", OwnerName: "someuser",
			Owner: &user_model.User{Name: "someuser", LowerName: "someuser"},
		},
		Ingredients: []*structs.Ingredient{
			{Identifier: "obs", Path: "./content", Title: "Histoires Bibliques", IsDir: true},
			{Identifier: "01", Path: "./content/01.md"},
		},
	}

	// the untitled, missing story is left to CheckOBSStories; the manifest's obs project is still checked
	issues := checkIngredients(t.Context(), dm)
	require.Len(t, issues, 1)
	assert.Equal(t, repo_model.IssueCodeIngredientMissing, issues[0].IssueCode)
	assert.Contains(t, issues[0].Details, "./content")
	assert.NotContains(t, issues[0].Details, "01.md")
}
