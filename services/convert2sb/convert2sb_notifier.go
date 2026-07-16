// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert2sb

import (
	"context"
	"strings"

	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	repo_module "gitea.dev/modules/repository"
	notify_service "gitea.dev/services/notify"
)

// sbFirstTopic is a hardcoded topic that, when added to a repo, triggers a one-time SB
// conversion. It bypasses the CONVERT2SB_TOPICS setting but still requires the repo to be
// an RC or ts repo on master. It is intentionally not used by the bulk cron task.
const sbFirstTopic = "sbfirst"

type convert2sbNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &convert2sbNotifier{}

// Init registers the Convert2SB notifier.
func Init(ctx context.Context) error {
	notify_service.RegisterNotifier(newNotifier())
	return nil
}

func newNotifier() notify_service.Notifier {
	return &convert2sbNotifier{}
}

// PushCommits triggers SB conversion when a push is made to the master branch of a qualifying repo.
func (n *convert2sbNotifier) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repo_module.PushUpdateOptions, commits *repo_module.PushCommits) {
	if repo == nil || opts == nil {
		return
	}
	if !opts.RefFullName.IsBranch() || opts.RefFullName.BranchName() != "master" {
		return
	}

	log.Info("Convert2SB notifier: push to master detected for %s, checking qualification", repo.FullName())

	qualifies, err := RepoQualifiesForConversion(ctx, repo)
	if err != nil {
		log.Warn("Convert2SB notifier: error checking qualification for %s: %v", repo.FullName(), err)
		return
	}
	if !qualifies {
		log.Info("Convert2SB notifier: %s does not qualify, skipping", repo.FullName())
		return
	}

	log.Info("Convert2SB notifier: spawning async conversion for %s branch master", repo.FullName())
	spawnConversion(repo)
}

// RepoTopicsChanged triggers SB conversion when a qualifying topic is added to a repo.
// Two cases trigger conversion:
//  1. The repo now has the hardcoded "sbfirst" topic — triggers if the repo is RC or ts on
//     master, regardless of the CONVERT2SB_TOPICS setting. This is a one-time manual trigger.
//  2. The repo now has a topic from CONVERT2SB_TOPICS — triggers via full qualification check.
func (n *convert2sbNotifier) RepoTopicsChanged(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	if repo == nil {
		return
	}

	// Reload the repo so repo.Topics reflects the just-saved topic list (the in-memory
	// object passed by the router was loaded before the topic update).
	freshRepo, err := repo_model.GetRepositoryByID(ctx, repo.ID)
	if err != nil {
		log.Error("Convert2SB notifier: GetRepositoryByID failed for repo ID %d: %v", repo.ID, err)
		return
	}

	log.Info("Convert2SB notifier: topics changed for %s, checking qualification", freshRepo.FullName())

	// Case 1: "sbfirst" topic — bypass CONVERT2SB_TOPICS, but still require RC or ts on master.
	for _, t := range freshRepo.Topics {
		if strings.EqualFold(t, sbFirstTopic) {
			if freshRepo.DefaultBranch != "master" {
				log.Info("Convert2SB notifier: %s has %q topic but default branch is %q, skipping",
					freshRepo.FullName(), sbFirstTopic, freshRepo.DefaultBranch)
				return
			}
			hasConvertible, err := repo_model.HasDefaultBranchConvertibleMetadata(ctx, freshRepo.ID)
			if err != nil {
				log.Warn("Convert2SB notifier: HasDefaultBranchConvertibleMetadata failed for %s: %v", freshRepo.FullName(), err)
				return
			}
			if !hasConvertible {
				log.Info("Convert2SB notifier: %s has %q topic but is not an RC or ts repo, skipping",
					freshRepo.FullName(), sbFirstTopic)
				return
			}
			log.Info("Convert2SB notifier: spawning async conversion for %s (sbfirst topic)", freshRepo.FullName())
			spawnConversion(freshRepo)
			return
		}
	}

	// Case 2: a CONVERT2SB_TOPICS topic — full qualification check.
	qualifies, err := RepoQualifiesForConversion(ctx, freshRepo)
	if err != nil {
		log.Warn("Convert2SB notifier: error checking qualification for %s: %v", freshRepo.FullName(), err)
		return
	}
	if !qualifies {
		log.Info("Convert2SB notifier: %s does not qualify after topic change, skipping", freshRepo.FullName())
		return
	}

	log.Info("Convert2SB notifier: spawning async conversion for %s (qualifying topic added)", freshRepo.FullName())
	spawnConversion(freshRepo)
}

func spawnConversion(repo *repo_model.Repository) {
	shutdownCtx := graceful.GetManager().ShutdownContext()
	go func(ctx context.Context, repo *repo_model.Repository) {
		select {
		case <-ctx.Done():
			log.Warn("Convert2SB: context canceled for %s branch master", repo.FullName())
			return
		default:
			if err := ForBranch(ctx, repo, "master"); err != nil {
				log.Error("Convert2SB: conversion failed for %s branch master: %v", repo.FullName(), err)
			}
		}
	}(shutdownCtx, repo)
}
