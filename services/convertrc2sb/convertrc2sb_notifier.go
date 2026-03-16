// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convertrc2sb

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	notify_service "code.gitea.io/gitea/services/notify"
)

type rc2sbNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &rc2sbNotifier{}

// Init registers the ConvertRC2SB notifier.
func Init(ctx context.Context) error {
	notify_service.RegisterNotifier(newNotifier())
	return nil
}

func newNotifier() notify_service.Notifier {
	return &rc2sbNotifier{}
}

// PushCommits triggers RC-to-SB conversion when a push is made to the master branch of a qualifying repo.
func (n *rc2sbNotifier) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repo_module.PushUpdateOptions, commits *repo_module.PushCommits) {
	if repo == nil || opts == nil {
		return
	}
	if !opts.RefFullName.IsBranch() || opts.RefFullName.BranchName() != "master" {
		return
	}

	log.Info("ConvertRC2SB notifier: push to master detected for %s, checking qualification", repo.FullName())

	qualifies, err := RepoQualifiesForConversion(ctx, repo)
	if err != nil {
		log.Warn("ConvertRC2SB notifier: error checking qualification for %s: %v", repo.FullName(), err)
		return
	}
	if !qualifies {
		log.Info("ConvertRC2SB notifier: %s does not qualify, skipping", repo.FullName())
		return
	}

	log.Info("ConvertRC2SB notifier: spawning async conversion for %s branch master", repo.FullName())
	shutdownCtx := graceful.GetManager().ShutdownContext()
	go func(ctx context.Context, repo *repo_model.Repository) {
		select {
		case <-ctx.Done():
			log.Warn("ConvertRC2SB: context canceled for %s branch master", repo.FullName())
			return
		default:
			if err := ForBranch(ctx, repo, "master"); err != nil {
				log.Error("ConvertRC2SB: conversion failed for %s branch master: %v", repo.FullName(), err)
			}
		}
	}(shutdownCtx, repo)
}
