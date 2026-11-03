// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/system"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/storage"
	door43metadata_service "gitea.dev/services/door43metadata"

	"github.com/urfave/cli/v3"
)

// CmdDoor43Metadata represents the available door43metadata sub-command.
var CmdDoor43Metadata = &cli.Command{
	Name:        "door43metadata",
	Usage:       "Scan repo(s) for the Door43 Metadata",
	Description: "Updates the Door43 Metadata of every repo, of the repos of the given owners, or of one repo",
	Action:      runDoor43Metadata,
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:    "owner",
			Aliases: []string{"o"},
			Usage:   `Owner whose repos to process; repeat it or separate names with commas for several. With "repo", the one owner of that repo`,
		},
		&cli.StringFlag{
			Name:    "repo",
			Aliases: []string{"r"},
			Value:   "",
			Usage:   `Name of a single repo to process; needs exactly one "owner"`,
		},
	},
}

// door43MetadataOwnerNames trims the --owner names and drops empty ones and repeats,
// ignoring case as owner names do.
func door43MetadataOwnerNames(names []string) []string {
	var owners []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" && !slices.ContainsFunc(owners, func(o string) bool { return strings.EqualFold(o, name) }) {
			owners = append(owners, name)
		}
	}
	return owners
}

func runDoor43Metadata(ctx context.Context, c *cli.Command) error {
	ownerNames := door43MetadataOwnerNames(c.StringSlice("owner"))
	if c.IsSet("owner") && len(ownerNames) == 0 {
		return errors.New("--owner(-o) names no owner") // not to be taken for a run over every repo
	}
	repoName := c.String("repo")
	if repoName != "" && len(ownerNames) != 1 {
		return errors.New("--repo(-r) needs exactly one --owner(-o)")
	}

	stdCtx, cancel := installSignals()
	defer cancel()

	if err := initDB(stdCtx); err != nil {
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	if repoName != "" {
		repo, err := repo_model.GetRepositoryByOwnerAndName(stdCtx, ownerNames[0], repoName)
		if err != nil {
			return err
		}
		return door43metadata_service.ProcessDoor43MetadataForRepo(stdCtx, repo, "")
	}

	if len(ownerNames) > 0 {
		owners := make([]*user_model.User, 0, len(ownerNames))
		for _, name := range ownerNames {
			owner, err := user_model.GetUserByName(stdCtx, name)
			if err != nil {
				return fmt.Errorf("owner %q: %w", name, err)
			}
			owners = append(owners, owner)
		}
		names := strings.Join(ownerNames, ", ")
		if err := system.CreateRepositoryNotice("Starting Door43 Metadata Update for the repos of %s", names); err != nil {
			return err
		}
		if err := door43metadata_service.UpdateDoor43Metadata(stdCtx, owners...); err != nil {
			return err
		}
		log.Info("Finished gathering the door43metadata for the repos of %s", names)
		return system.CreateRepositoryNotice("Finished Door43 Metadata Update for the repos of %s", names)
	}

	if err := system.CreateRepositoryNotice("Starting FULL Door43 Metadata Update - PROCESSING ALL REPOS AND REFS"); err != nil {
		return err
	}

	if err := door43metadata_service.UpdateDoor43Metadata(stdCtx); err != nil {
		return err
	}

	log.Info("Finished gathering the door43metadata for all repos")
	return system.CreateRepositoryNotice("FINISHED FULL Door43 Metadata Update - PROCESSED ALL REPOS AND REFS")
}
