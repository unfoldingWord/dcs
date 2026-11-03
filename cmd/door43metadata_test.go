// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
}
