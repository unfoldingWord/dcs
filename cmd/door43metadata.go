// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
	door43metadata_service "code.gitea.io/gitea/services/door43metadata"

	"github.com/urfave/cli/v3"
)

// CmdDoor43Metadata represents the available door43metadata sub-command.
var CmdDoor43Metadata = &cli.Command{
	Name:        "door43metadata",
	Usage:       "Scan repo(s) for the Door43 Metadata",
	Description: "A command to update all repos or a repo's Door43 Metadata",
	Action:      runDoor43Metadata,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "owner",
			Aliases: []string{"o"},
			Value:   "",
			Usage:   `Name of a the owner of the repo (see repo argument) to generate the door43metadata. "repo" must be set as well`,
		},
		&cli.StringFlag{
			Name:    "repo",
			Aliases: []string{"r"},
			Value:   "",
			Usage:   `Name of a single repo to generate the door43metadata. "owner" must also be set for this to be accepted`,
		},
	},
}

func runDoor43Metadata(ctx context.Context, c *cli.Command) error {
	ownerName := c.String("owner")
	repoName := c.String("repo")
	if ownerName != "" && repoName == "" {
		return fmt.Errorf("--repo(-r) must be specified if --owner(-o) is used")
	}
	if ownerName == "" && repoName != "" {
		return fmt.Errorf("--owner(-o) must be supplied if --repo(-r) is used")
	}

	stdCtx, cancel := installSignals()
	defer cancel()

	if err := initDB(stdCtx); err != nil {
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	if ownerName != "" && repoName != "" {
		repo, err := repo_model.GetRepositoryByOwnerAndName(stdCtx, ownerName, repoName)
		if err != nil {
			return err
		}
		return door43metadata_service.ProcessDoor43MetadataForRepo(stdCtx, repo, "")
	}

	if err := system.CreateRepositoryNotice("Starting FULL Door43 Metadata Update - PROCESSING ALL REPOS AND REFS"); err != nil {
		return err
	}

	err := door43metadata_service.UpdateDoor43Metadata(stdCtx)
	if err != nil {
		return err
	}

	log.Info("Finished gathering the door43metadata for all repos")
	if err := system.CreateRepositoryNotice("FINSIEHD FULL Door43 Metadata Update - PROCESSED ALL REPOS AND REFS"); err != nil {
		return err
	}

	return nil
}
