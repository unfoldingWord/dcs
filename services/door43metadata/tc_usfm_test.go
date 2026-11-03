// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPopulateTcDoor43Metadata_USFM covers how a tC entry finds its book: the exported USFM
// go-tc2rc converts, or, in a save from before tc_version, the chapter JSON it converts unaligned.
func TestPopulateTcDoor43Metadata_USFM(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()

	repo, err := repo_model.GetRepositoryByID(ctx, 1)
	require.NoError(t, err)
	gitRepo, err := git.OpenRepository(ctx, repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	branchCommit := func(t *testing.T, branch string, files map[string]string) *git.Commit {
		commitFilesOnBranch(t, gitrepo.RepoLocalPath(repo.CodeStorageRepo()), branch, files)
		commit, err := gitRepo.GetBranchCommit(ctx, branch)
		require.NoError(t, err)
		return commit
	}

	t.Run("a save with chapter JSON and no USFM", func(t *testing.T) {
		manifest := `{"package_version": 1, "generator": {"name": "tc-desktop"}, "project": {"id": "tit", "name": "Titus"}, "target_language": {"id": "en", "name": "English"}}`
		commit := branchCommit(t, "dcs-tc-save", map[string]string{
			"manifest.json": manifest,
			"tit/1.json":    `{"1": "Paul, a servant of God"}`,
		})
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateTcTsDoor43Metadata(ctx, gitRepo, dm, repo, commit, []byte(manifest)))
		assert.Nil(t, dm.ValidationError)
		assert.Equal(t, "tc", dm.MetadataType)
		assert.Equal(t, "Bible", dm.Subject, "go-tc2rc converts chapter JSON unaligned, as a Bible")
		assert.Equal(t, "textTranslation", dm.Flavor)
		require.Len(t, dm.Ingredients, 1)
		assert.Equal(t, "./tit", dm.Ingredients[0].Path)
		assert.True(t, dm.CanConvertToSB())
	})

	tcManifest := `{"tc_version": 8, "project": {"id": "tit"}, "target_language": {"id": "en"}}`
	populate := func(t *testing.T, branch, manifest string, files map[string]string) *repo_model.Door43Metadata {
		t.Helper()
		files["manifest.json"] = manifest
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateTcTsDoor43Metadata(ctx, gitRepo, dm, repo, branchCommit(t, branch, files), []byte(manifest)))
		require.Nil(t, dm.ValidationError)
		require.Len(t, dm.Ingredients, 1)
		return dm
	}

	t.Run("USFM named after the book wins over another USFM, as in go-tc2rc", func(t *testing.T) {
		dm := populate(t, "dcs-tc-usfm", tcManifest, map[string]string{
			"57-TIT.usfm": "\\id TIT\n\\c 1\n\\v 1 Paul\n",
			"tit.usfm":    "\\id TIT\n\\c 1\n\\v 1 Paul\n",
		})
		assert.Equal(t, "Aligned Bible", dm.Subject)
		assert.Equal(t, "./tit.usfm", dm.Ingredients[0].Path)
	})

	t.Run("the only USFM, named after neither", func(t *testing.T) {
		dm := populate(t, "dcs-tc-usfm-other", tcManifest, map[string]string{"57-TIT.usfm": "\\id TIT\n"})
		assert.Equal(t, "./57-TIT.usfm", dm.Ingredients[0].Path)
	})

	t.Run("the chapter folder keeps its own case, as go-tc2rc finds it", func(t *testing.T) {
		manifest := `{"package_version": 1, "generator": {"name": "tc-desktop"}, "project": {"id": "tit"}, "target_language": {"id": "en"}}`
		dm := populate(t, "dcs-tc-save-upper", manifest, map[string]string{"TIT/1.json": `{"1": "Paul"}`})
		assert.Equal(t, "Bible", dm.Subject)
		assert.Equal(t, "./TIT", dm.Ingredients[0].Path)
		assert.True(t, dm.Ingredients[0].Exists)
		assert.True(t, dm.Ingredients[0].IsDir)
	})

	t.Run("no USFM and no chapter JSON keeps the manifest's subject", func(t *testing.T) {
		dm := populate(t, "dcs-tc-bare", tcManifest, map[string]string{"tit/notes.md": "not a chapter"})
		assert.Equal(t, "Aligned Bible", dm.Subject, "go-tc2rc builds unaligned text only from <chapter>.json files")
		assert.Equal(t, "./repo1.usfm", dm.Ingredients[0].Path)
	})

	t.Run("a tS project with no USFM is not a tC save", func(t *testing.T) {
		manifest := `{"package_version": 6, "type": {"id": "text"}, "project": {"id": "tit"}, "resource": {"id": "ulb"}, "target_language": {"id": "en"}}`
		dm := populate(t, "dcs-ts-text", manifest, map[string]string{"01/01.txt": "\\v 1 Paul"})
		assert.Equal(t, "ts", dm.MetadataType)
		assert.Equal(t, "Bible", dm.Subject)
		assert.Equal(t, ".", dm.Ingredients[0].Path)
	})

	t.Run("a USFM named after the project wins over one sorted first", func(t *testing.T) {
		manifest := `{"tc_version": 8, "project": {"id": "jon"}, "target_language": {"id": "en"}}`
		commit := branchCommit(t, "dcs-tc-usfm-project", map[string]string{
			"manifest.json": manifest,
			"32-JON.usfm":   "\\id JON\n\\c 1\n\\v 1 Now\n",
			"jon.usfm":      "\\id JON\n\\c 1\n\\v 1 Now\n",
		})
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateTcTsDoor43Metadata(ctx, gitRepo, dm, repo, commit, []byte(manifest)))
		require.Len(t, dm.Ingredients, 1)
		assert.Equal(t, "./jon.usfm", dm.Ingredients[0].Path)
	})
}
