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
	convert2sb_service "gitea.dev/services/convert2sb"

	"github.com/urfave/cli/v3"
)

// CmdConvert2SB represents the available convert2sb sub-command.
var CmdConvert2SB = &cli.Command{
	Name:        "convert2sb",
	Aliases:     []string{"convertrc2sb"}, // deprecated name, kept for existing scripts
	Usage:       "Convert RC and tS repos to Scripture Burrito format",
	Description: "Converts qualifying RC and translationStudio (ts) repositories to SB format and pushes to the main branch. ts repos are first converted to RC format.",
	Action:      runConvert2SB,
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

func runConvert2SB(ctx context.Context, c *cli.Command) error {
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

		qualifies, err := convert2sb_service.RepoQualifiesForConversion(stdCtx, repo)
		if err != nil {
			return fmt.Errorf("error checking qualification: %w", err)
		}
		if !qualifies {
			return fmt.Errorf("repository %s/%s does not qualify for SB conversion (requires MetadataType=rc or ts, DefaultBranch=master, and a qualifying topic)", ownerName, repoName)
		}

		return convert2sb_service.ForBranch(stdCtx, repo, repo.DefaultBranch)
	}

	if err := system.CreateRepositoryNotice("Starting FULL RC/tS-to-SB Conversion - PROCESSING ALL QUALIFYING REPOS"); err != nil {
		return err
	}

	if err := convert2sb_service.Convert2SBAllRepos(stdCtx); err != nil {
		return err
	}

	log.Info("Finished RC/tS-to-SB conversion for all qualifying repos")
	return system.CreateRepositoryNotice("FINISHED FULL RC/tS-to-SB Conversion - PROCESSED ALL QUALIFYING REPOS")
}
