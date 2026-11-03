// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	repo_model "gitea.dev/models/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// TestDoor43MetadataArgs checks the arguments that fail before the database is opened.
func TestDoor43MetadataArgs(t *testing.T) {
	for _, args := range [][]string{
		{"--repo", "en_tn"},
		{"--owner", "unfoldingWord", "--owner", "Door43-Catalog", "--repo", "en_tn"},
	} {
		cmd := &cli.Command{Flags: door43MetadataFlags(), Action: runDoor43Metadata}
		err := cmd.Run(t.Context(), append([]string{"door43metadata"}, args...))
		assert.EqualError(t, err, "--repo(-r) needs exactly one --owner(-o)", "%q", args)
	}
	for _, owner := range []string{" ", ",", " , "} {
		cmd := &cli.Command{Flags: door43MetadataFlags(), Action: runDoor43Metadata}
		err := cmd.Run(t.Context(), []string{"door43metadata", "--owner", owner})
		assert.EqualError(t, err, "--owner(-o) names no owner", "a blank --owner must not become a full run: %q", owner)
	}
	cmd := &cli.Command{Flags: door43MetadataFlags(), Action: runDoor43Metadata}
	assert.EqualError(t, cmd.Run(t.Context(), []string{"door43metadata", "--topic", " , "}), "--topic names no topic")
	for _, severity := range []string{"0", "5", "-1"} {
		cmd := &cli.Command{Flags: door43MetadataFlags(), Action: runDoor43Metadata}
		err := cmd.Run(t.Context(), []string{"door43metadata", "--healthcheck-severity", severity})
		assert.EqualError(t, err, "--healthcheck-severity must be 1 (success), 2 (info), 3 (warning) or 4 (error)", severity)
	}
}

func TestDoor43MetadataNoneSelected(t *testing.T) {
	assert.EqualError(t, door43MetadataNoneSelected([]string{"unfoldingWord"}, "en_tn", 0),
		"unfoldingWord/en_tn was not processed: with a filter, only a public, unarchived repo matching every flag is")
	assert.NoError(t, door43MetadataNoneSelected([]string{"unfoldingWord"}, "en_tn", 1))
	assert.NoError(t, door43MetadataNoneSelected([]string{"unfoldingWord"}, "", 0), "a filter over many repos may match none")
}

func TestDoor43MetadataScope(t *testing.T) {
	assert.Empty(t, door43MetadataScope(nil, repo_model.ReposForMetadataOptions{}), "no flags: every repo")
	assert.Equal(t, "of ru_gl, es-419_gl, with a healthcheck severity of warning or worse, with a validation error, with a release, with the topics tc-ready",
		door43MetadataScope([]string{"ru_gl", "es-419_gl"}, repo_model.ReposForMetadataOptions{
			OwnerIDs: []int64{1, 2}, MinHealthcheckSeverity: repo_model.SeverityLevelWarning, HasValidationError: true, HasRelease: true, Topics: []string{"tc-ready"},
		}))
	assert.Equal(t, "unfoldingWord/en_tn, with a release", door43MetadataScope([]string{"unfoldingWord"}, repo_model.ReposForMetadataOptions{RepoName: "en_tn", HasRelease: true}))
}

func TestDoor43MetadataOwnerNames(t *testing.T) {
	var got []string
	cmd := &cli.Command{Flags: door43MetadataFlags(), Action: func(_ context.Context, c *cli.Command) error {
		got = door43MetadataNames(c.StringSlice("owner"))
		return nil
	}}
	require.NoError(t, cmd.Run(t.Context(), []string{"door43metadata", "--owner", "ru_gl, es-419_gl", "-o", "RU_GL", "-o", " "}))
	assert.Equal(t, []string{"ru_gl", "es-419_gl"}, got, "comma-separated, trimmed, repeats and empty names dropped regardless of case")
}
