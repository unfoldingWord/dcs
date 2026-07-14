// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/system"
	"gitea.dev/modules/log"
	"gitea.dev/modules/storage"
	convertrc2sb_service "gitea.dev/services/convertrc2sb"

	"github.com/urfave/cli/v3"
)

// CmdConvertRC2SB represents the available convertrc2sb sub-command.
var CmdConvertRC2SB = &cli.Command{
	Name:        "convertrc2sb",
	Usage:       "Convert RC repos to Scripture Burrito format",
	Description: "Converts qualifying RC repositories to SB format and pushes to the main branch",
	Action:      runConvertRC2SB,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "owner",
			Aliases: []string{"o"},
			Value:   "",
			Usage:   `Name of the owner of the repo to convert. "repo" must be set as well`,
		},
		&cli.StringFlag{
			Name:    "repo",
			Aliases: []string{"r"},
			Value:   "",
			Usage:   `Name of a single repo to convert. "owner" must also be set`,
		},
	},
}

func runConvertRC2SB(ctx context.Context, c *cli.Command) error {
	ownerName := c.String("owner")
	repoName := c.String("repo")
	if ownerName != "" && repoName == "" {
		return errors.New("--repo(-r) must be specified if --owner(-o) is used")
	}
	if ownerName == "" && repoName != "" {
		return errors.New("--owner(-o) must be supplied if --repo(-r) is used")
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

		qualifies, err := convertrc2sb_service.RepoQualifiesForConversion(stdCtx, repo)
		if err != nil {
			return fmt.Errorf("error checking qualification: %w", err)
		}
		if !qualifies {
			return fmt.Errorf("repository %s/%s does not qualify for RC-to-SB conversion (requires MetadataType=rc, DefaultBranch=master, and a qualifying topic)", ownerName, repoName)
		}

		return convertrc2sb_service.ForBranch(stdCtx, repo, repo.DefaultBranch)
	}

	if err := system.CreateRepositoryNotice("Starting FULL RC-to-SB Conversion - PROCESSING ALL QUALIFYING REPOS"); err != nil {
		return err
	}

	if err := convertrc2sb_service.AllRepos(stdCtx); err != nil {
		return err
	}

	log.Info("Finished RC-to-SB conversion for all qualifying repos")
	return system.CreateRepositoryNotice("FINISHED FULL RC-to-SB Conversion - PROCESSED ALL QUALIFYING REPOS")
}
