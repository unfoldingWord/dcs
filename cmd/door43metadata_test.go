// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

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
		cmd := &cli.Command{Flags: CmdDoor43Metadata.Flags, Action: runDoor43Metadata}
		err := cmd.Run(t.Context(), append([]string{"door43metadata"}, args...))
		assert.EqualError(t, err, "--repo(-r) needs exactly one --owner(-o)", "%q", args)
	}
	for _, owner := range []string{" ", ",", " , "} {
		cmd := &cli.Command{Flags: CmdDoor43Metadata.Flags, Action: runDoor43Metadata}
		err := cmd.Run(t.Context(), []string{"door43metadata", "--owner", owner})
		assert.EqualError(t, err, "--owner(-o) names no owner", "a blank --owner must not become a full run: %q", owner)
	}
}

func TestDoor43MetadataOwnerNames(t *testing.T) {
	var got []string
	cmd := &cli.Command{Flags: CmdDoor43Metadata.Flags, Action: func(_ context.Context, c *cli.Command) error {
		got = door43MetadataOwnerNames(c.StringSlice("owner"))
		return nil
	}}
	require.NoError(t, cmd.Run(t.Context(), []string{"door43metadata", "--owner", "ru_gl, es-419_gl", "-o", "RU_GL", "-o", " "}))
	assert.Equal(t, []string{"ru_gl", "es-419_gl"}, got, "comma-separated, trimmed, repeats and empty names dropped regardless of case")
}
