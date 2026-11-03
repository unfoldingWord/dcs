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
	Description: "Updates the Door43 Metadata of public repos: all of them with no flags, or only those that match every flag given",
	Action:      runDoor43Metadata,
	Flags:       door43MetadataFlags(),
}

// door43MetadataFlags returns new flags each call: a flag keeps having been set, so tests
// running the command more than once need their own.
func door43MetadataFlags() []cli.Flag {
	return []cli.Flag{
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
		&cli.IntFlag{
			Name:  "healthcheck-severity",
			Usage: "Only repos with a ref whose healthcheck severity is at least this: 1 success, 2 info, 3 warning, 4 error",
		},
		&cli.BoolFlag{
			Name:  "has-validation-error",
			Usage: "Only repos with a ref whose metadata file couldn't be read or validated",
		},
		&cli.BoolFlag{
			Name:  "has-release",
			Usage: "Only repos with a published release, not a draft or a bare tag",
		},
		&cli.StringSliceFlag{
			Name:  "topic",
			Usage: "Only repos with this topic; repeat it or separate topics with commas, and a repo needs them all",
		},
	}
}

// door43MetadataNames trims the names given to a flag and drops empty ones and repeats,
// ignoring case as owner names and topics do.
func door43MetadataNames(names []string) []string {
	var kept []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" && !slices.ContainsFunc(kept, func(k string) bool { return strings.EqualFold(k, name) }) {
			kept = append(kept, name)
		}
	}
	return kept
}

// door43MetadataFiltered reports whether opts narrows the repos beyond their owners and name.
func door43MetadataFiltered(opts repo_model.ReposForMetadataOptions) bool {
	return opts.MinHealthcheckSeverity > 0 || opts.HasValidationError || opts.HasRelease || len(opts.Topics) > 0
}

// door43MetadataScope describes the repos the flags select, for the notices, or "" for all of them.
func door43MetadataScope(ownerNames []string, opts repo_model.ReposForMetadataOptions) string {
	var parts []string
	switch {
	case opts.RepoName != "":
		parts = append(parts, ownerNames[0]+"/"+opts.RepoName)
	case len(ownerNames) > 0:
		parts = append(parts, "of "+strings.Join(ownerNames, ", "))
	}
	if opts.MinHealthcheckSeverity > 0 {
		parts = append(parts, fmt.Sprintf("with a healthcheck severity of %s or worse", opts.MinHealthcheckSeverity))
	}
	if opts.HasValidationError {
		parts = append(parts, "with a validation error")
	}
	if opts.HasRelease {
		parts = append(parts, "with a release")
	}
	if len(opts.Topics) > 0 {
		parts = append(parts, "with the topics "+strings.Join(opts.Topics, ", "))
	}
	return strings.Join(parts, ", ")
}

func runDoor43Metadata(ctx context.Context, c *cli.Command) error {
	ownerNames := door43MetadataNames(c.StringSlice("owner"))
	if c.IsSet("owner") && len(ownerNames) == 0 {
		return errors.New("--owner(-o) names no owner") // not to be taken for a run over every repo
	}
	topics := door43MetadataNames(c.StringSlice("topic"))
	if c.IsSet("topic") && len(topics) == 0 {
		return errors.New("--topic names no topic") // not to be taken for no topic filter
	}
	repoName := strings.TrimSpace(c.String("repo"))
	if repoName != "" && len(ownerNames) != 1 {
		return errors.New("--repo(-r) needs exactly one --owner(-o)")
	}
	severity := c.Int("healthcheck-severity")
	if c.IsSet("healthcheck-severity") && (severity < int(repo_model.SeverityLevelSuccess) || severity > int(repo_model.SeverityLevelError)) {
		return errors.New("--healthcheck-severity must be 1 (success), 2 (info), 3 (warning) or 4 (error)")
	}
	opts := repo_model.ReposForMetadataOptions{
		RepoName:               repoName,
		MinHealthcheckSeverity: repo_model.SeverityLevel(severity),
		HasValidationError:     c.Bool("has-validation-error"),
		HasRelease:             c.Bool("has-release"),
		Topics:                 topics,
	}

	stdCtx, cancel := installSignals()
	defer cancel()

	if err := initDB(stdCtx); err != nil {
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	// One repo and nothing else to match: processed even when private, which purges its metadata
	if repoName != "" && !door43MetadataFiltered(opts) {
		repo, err := repo_model.GetRepositoryByOwnerAndName(stdCtx, ownerNames[0], repoName)
		if err != nil {
			return err
		}
		return door43metadata_service.ProcessDoor43MetadataForRepo(stdCtx, repo, "")
	}

	for _, name := range ownerNames {
		owner, err := user_model.GetUserByName(stdCtx, name)
		if err != nil {
			return fmt.Errorf("owner %q: %w", name, err)
		}
		opts.OwnerIDs = append(opts.OwnerIDs, owner.ID)
	}

	scope := door43MetadataScope(ownerNames, opts)
	if scope == "" {
		if err := system.CreateRepositoryNotice("Starting FULL Door43 Metadata Update - PROCESSING ALL REPOS AND REFS"); err != nil {
			return err
		}
		if _, err := door43metadata_service.UpdateDoor43Metadata(stdCtx, opts); err != nil {
			return err
		}
		log.Info("Finished gathering the door43metadata for all repos")
		return system.CreateRepositoryNotice("FINISHED FULL Door43 Metadata Update - PROCESSED ALL REPOS AND REFS")
	}

	if err := system.CreateRepositoryNotice("Starting Door43 Metadata Update for the repos %s", scope); err != nil {
		return err
	}
	n, err := door43metadata_service.UpdateDoor43Metadata(stdCtx, opts)
	if err != nil {
		return err
	}
	if err = door43MetadataNoneSelected(ownerNames, repoName, n); err != nil {
		return err
	}
	log.Info("Finished gathering the door43metadata for %d repos %s", n, scope)
	return system.CreateRepositoryNotice("Finished Door43 Metadata Update for %d repos %s", n, scope)
}

// door43MetadataNoneSelected fails a run for a named repo that the filters left out, so it isn't taken for done.
func door43MetadataNoneSelected(ownerNames []string, repoName string, n int) error {
	if repoName != "" && n == 0 {
		return fmt.Errorf("%s/%s was not processed: with a filter, only a public, unarchived repo matching every flag is", ownerNames[0], repoName)
	}
	return nil
}
