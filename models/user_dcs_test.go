// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetRepoMetadataFacets checks the owner rollup cached on the user table: the
// distinct languages, subjects and metadata types of the owner's public, unarchived
// repo-metadata entries, each alphabetized and without empty values.
func TestGetRepoMetadataFacets(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	dms := []*repo_model.Door43Metadata{
		// repo1 (user2, public)
		{
			RepoID: 1, Ref: "master", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000001",
			Stage: door43metadata.StageProd, IsLatestForStage: true, IsRepoMetadata: true,
			Language: "en", Subject: "Open Bible Stories", MetadataType: "rc",
		},
		// repo33 "utf8" (user2, public) - a second value for each field
		{
			RepoID: 33, Ref: "master", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000033",
			Stage: door43metadata.StagePreProd, IsLatestForStage: true, IsRepoMetadata: true,
			Language: "es-419", Subject: "Aligned Bible", MetadataType: "sb",
		},
		// repo42 "glob" (user2, public) - duplicates repo1's values, and has no subject
		{
			RepoID: 42, Ref: "master", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000042",
			Stage: door43metadata.StageProd, IsLatestForStage: true, IsRepoMetadata: true,
			Language: "en", Subject: "", MetadataType: "rc",
		},
		// repo1 again, but not the repo metadata entry
		{
			RepoID: 1, Ref: "v1", RefType: "tag", CommitSHA: "000000000000000000000000000000000000000a",
			Stage: door43metadata.StageProd, IsLatestForStage: false, IsRepoMetadata: false,
			Language: "fr", Subject: "TSV Translation Notes", MetadataType: "tc",
		},
		// repo2 (user2, private)
		{
			RepoID: 2, Ref: "master", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000002",
			Stage: door43metadata.StageProd, IsLatestForStage: true, IsRepoMetadata: true,
			Language: "de", Subject: "Bible", MetadataType: "ts",
		},
		// repo3 (org3, public) - another owner
		{
			RepoID: 3, Ref: "master", RefType: "branch", CommitSHA: "0000000000000000000000000000000000000003",
			Stage: door43metadata.StageProd, IsLatestForStage: true, IsRepoMetadata: true,
			Language: "hbo", Subject: "Hebrew Old Testament", MetadataType: "rc",
		},
	}
	for _, dm := range dms {
		_, err := db.GetEngine(t.Context()).Insert(dm)
		require.NoError(t, err)
	}

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	languages, subjects, metadataTypes, err := GetRepoMetadataFacets(t.Context(), user2)
	require.NoError(t, err)
	assert.Equal(t, []string{"en", "es-419"}, languages)
	assert.Equal(t, []string{"Aligned Bible", "Open Bible Stories"}, subjects)
	assert.Equal(t, []string{"rc", "sb"}, metadataTypes)

	// An owner with no metadata gets empty lists rather than nils from a failed query
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	languages, subjects, metadataTypes, err = GetRepoMetadataFacets(t.Context(), user1)
	require.NoError(t, err)
	assert.Empty(t, languages)
	assert.Empty(t, subjects)
	assert.Empty(t, metadataTypes)
}
