// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
	door43metadata_service "code.gitea.io/gitea/services/door43metadata"

	"github.com/urfave/cli"
)

// CmdDoor43Metadata represents the available door43metadata sub-command.
var CmdDoor43Metadata = cli.Command{
	Name:        "door43metadata",
	Usage:       "Scan repo(s) for the Door43 Metadata",
	Description: "A command to update all repos or a repo's Door43 Metadata",
	Action:      runDoor43Metadata,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "owner",
			Usage: `Name of a the owner of the repo (see repo argument) to generate the door43metadata. "repo" must be set as well`,
		},
		cli.StringFlag{
			Name:  "repo",
			Usage: `Name of a single repo to generate the door43metadata. "owner" must also be set for this to be accepted`,
		},
	},
}

func runDoor43Metadata(ctx *cli.Context) error {
	ownerName := ctx.String("owner")
	repoName := ctx.String("repo")
	if ownerName != "" && repoName == "" {
		return fmt.Errorf("--repo must be specified if --owner is used")
	}
	if ownerName == "" && repoName != "" {
		return fmt.Errorf("--owner must be supplied if --repo is used")
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

	err := door43metadata_service.UpdateDoor43Metadata(stdCtx)
	if err != nil {
		return err
	}

	if repoName != "" {
		log.Info("Finished gathering the door43metadata for %s/%s", ownerName, repoName)
	} else {
		log.Info("Finished gathering the door43metadaa for all repos")
	}

	return nil
}
