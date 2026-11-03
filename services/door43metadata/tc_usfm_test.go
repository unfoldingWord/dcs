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

	t.Run("an exported USFM named after the book", func(t *testing.T) {
		manifest := `{"tc_version": 8, "project": {"id": "tit"}, "target_language": {"id": "en"}}`
		commit := branchCommit(t, "dcs-tc-usfm", map[string]string{
			"manifest.json": manifest,
			"tit.usfm":      "\\id TIT\n\\c 1\n\\v 1 Paul\n",
		})
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateTcTsDoor43Metadata(ctx, gitRepo, dm, repo, commit, []byte(manifest)))
		assert.Equal(t, "Aligned Bible", dm.Subject)
		require.Len(t, dm.Ingredients, 1)
		assert.Equal(t, "./tit.usfm", dm.Ingredients[0].Path, "go-tc2rc takes the only .usfm when none is named after the repo")
	})
}
