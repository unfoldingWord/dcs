// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"fmt"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/dcs"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rcOBSManifest = `dublin_core:
  conformsto: rc0.2
  format: text/markdown
  identifier: obs
  language: {identifier: fr, title: Français, direction: ltr}
  subject: Open Bible Stories
  title: Histoires Bibliques
checking: {checking_level: '1'}
projects:
  - identifier: obs
    path: ./content
    title: Histoires Bibliques
`

const tsOBSManifest = `{"package_version": 7, "format": "markdown",
  "target_language": {"id": "fr", "name": "Français", "direction": "ltr"},
  "project": {"id": "obs", "name": "Open Bible Stories"}, "resource": {"id": "obs", "name": "Open Bible Stories"}}`

func summarizeIngredients(ingredients []*structs.Ingredient) []string {
	summary := make([]string, 0, len(ingredients))
	for _, ing := range ingredients {
		summary = append(summary, fmt.Sprintf("%s %s %q sort=%d dir=%t exists=%t", ing.Identifier, ing.Path, ing.Title, ing.Sort, ing.IsDir, ing.Exists))
	}
	return summary
}

func TestOBSIngredients(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()

	repo, err := repo_model.GetRepositoryByID(ctx, 1)
	require.NoError(t, err)
	repoPath := gitrepo.RepoLocalPath(repo.CodeStorageRepo())
	gitRepo, err := git.OpenRepository(ctx, repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	branchCommit := func(t *testing.T, branch string, files map[string]string) *git.Commit {
		commitFilesOnBranch(t, repoPath, branch, files)
		commit, err := gitRepo.GetBranchCommit(ctx, branch)
		require.NoError(t, err)
		return commit
	}

	t.Run("rc lists the content dir after the obs project", func(t *testing.T) {
		commit := branchCommit(t, "dcs-obs-rc", map[string]string{
			"manifest.yaml":          rcOBSManifest,
			"content/01.md":          "\ufeff# 1. La Création\r\n\r\n![OBS Image](https://cdn.door43.org/obs/jpg/360px/obs-en-01-01.jpg)\n",
			"content/02.md":          "![OBS Image](https://cdn.door43.org/obs/jpg/360px/obs-en-02-01.jpg)\n",
			"content/51.md":          "# 51. Not a story\n",
			"content/front/title.md": "Histoires Bibliques",
			"content/back/intro.md":  "## Au sujet\n",
		})
		manifest, err := dcs.ParseYAML([]byte(rcOBSManifest))
		require.NoError(t, err)
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, GetDoor43MetadataFromRCManifest(ctx, gitRepo, dm, manifest, repo, commit))
		assert.Equal(t, []string{
			`obs ./content "Histoires Bibliques" sort=0 dir=true exists=true`,
			`front ./content/front "Front Matter" sort=0 dir=true exists=true`,
			`01 ./content/01.md "1. La Création" sort=1 dir=false exists=true`,
			`02 ./content/02.md "" sort=2 dir=false exists=true`,
			`back ./content/back "Back Matter" sort=51 dir=true exists=true`,
		}, summarizeIngredients(dm.Ingredients))
		assert.Positive(t, dm.Ingredients[2].Size)
		assert.Equal(t, []string{"obs", "front", "01", "02", "back"}, dm.IngredientsIdentifierList())
	})

	t.Run("ts lists the story dirs in the repo root", func(t *testing.T) {
		commit := branchCommit(t, "dcs-obs-ts", map[string]string{
			"manifest.json":   tsOBSManifest,
			"LICENSE.md":      "license",
			"front/title.txt": "Histoires Bibliques",
			"01/title.txt":    "1. La Création",
			"01/01.txt":       "Au commencement...",
			"02/01.txt":       "Adam et sa femme...",
		})
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateTcTsDoor43Metadata(ctx, gitRepo, dm, repo, commit, []byte(tsOBSManifest)))
		assert.Equal(t, []string{
			`obs . "Open Bible Stories" sort=0 dir=true exists=true`,
			`front ./front "Front Matter" sort=0 dir=true exists=true`,
			`01 ./01 "1. La Création" sort=1 dir=true exists=true`,
			`02 ./02 "" sort=2 dir=true exists=true`,
		}, summarizeIngredients(dm.Ingredients))
	})

	sbOBS := func(ingredientPaths ...string) *dcs.SBMetadata100 {
		sb := &dcs.SBMetadata100{
			Identification: &dcs.SB100Identification{Name: dcs.LocalizedText{"fr": "Histoires Bibliques"}},
			Languages:      []*dcs.SB100Language{{Tag: "fr"}},
			Type:           &dcs.SB100Type{FlavorType: dcs.SB100FlavorType{Name: "gloss", Flavor: dcs.SB100Flavor{Name: "textStories"}}},
			Ingredients:    map[string]*dcs.SB100Ingredient{},
		}
		for _, p := range ingredientPaths {
			sb.Ingredients[p] = &dcs.SB100Ingredient{}
		}
		return sb
	}

	t.Run("sb lists what metadata.json declares, as rc2sb lays it out", func(t *testing.T) {
		commit := branchCommit(t, "dcs-obs-sb", map[string]string{
			"metadata.json":                      "{}",
			"ingredients/LICENSE.md":             "license",
			"ingredients/content/01.md":          "# 1. La Création\n",
			"ingredients/content/03.md":          "# 3. Not declared\n",
			"ingredients/content/front/intro.md": "# Histoires Bibliques\n",
			"ingredients/content/front/title.md": "Histoires Bibliques",
		})
		sb := sbOBS("ingredients/LICENSE.md", "ingredients/content/01.md", "ingredients/content/02.md",
			"ingredients/content/front/intro.md", "ingredients/content/front/title.md")
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, GetDoor43MetadataFromSBMetadata(ctx, gitRepo, dm, sb, repo, commit))
		assert.Equal(t, "Open Bible Stories", dm.Subject)
		assert.Equal(t, []string{
			`obs ./ingredients "Histoires Bibliques" sort=0 dir=true exists=true`,
			`front ./ingredients/content/front "Front Matter" sort=0 dir=true exists=true`,
			`01 ./ingredients/content/01.md "1. La Création" sort=1 dir=false exists=true`,
			`02 ./ingredients/content/02.md "" sort=2 dir=false exists=false`,
		}, summarizeIngredients(dm.Ingredients))
	})

	t.Run("sb with Scribe's flat layout", func(t *testing.T) {
		sb := sbOBS("ingredients/01.md", "ingredients/front.md", "ingredients/back.md", "ingredients/scribe-settings.json")
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, GetDoor43MetadataFromSBMetadata(ctx, nil, dm, sb, repo, nil))
		assert.Equal(t, []string{
			`obs ./ingredients "Histoires Bibliques" sort=0 dir=true exists=true`,
			`front ./ingredients/front.md "Front Matter" sort=0 dir=false exists=false`,
			`01 ./ingredients/01.md "" sort=1 dir=false exists=false`,
			`back ./ingredients/back.md "Back Matter" sort=51 dir=false exists=false`,
		}, summarizeIngredients(dm.Ingredients))
	})
}
