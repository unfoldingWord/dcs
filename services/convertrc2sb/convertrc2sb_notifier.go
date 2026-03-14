// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convertrc2sb

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
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

func (n *rc2sbNotifier) NewRelease(ctx context.Context, rel *repo_model.Release) {
	if rel == nil || rel.IsDraft {
		return
	}
	n.maybeConvert(ctx, rel)
}

func (n *rc2sbNotifier) UpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	if rel == nil || rel.IsDraft {
		return
	}
	n.maybeConvert(ctx, rel)
}

func (n *rc2sbNotifier) NewTagRelease(ctx context.Context, rel *repo_model.Release) {
	n.NewRelease(ctx, rel)
}

// maybeConvert checks if the repo qualifies and spawns the conversion in a goroutine.
func (n *rc2sbNotifier) maybeConvert(ctx context.Context, rel *repo_model.Release) {
	// Ensure the release has its repo loaded
	if rel.Repo == nil {
		if err := rel.LoadAttributes(ctx); err != nil {
			log.Error("ConvertRC2SB notifier: LoadAttributes failed for release ID %d: %v", rel.ID, err)
			return
		}
	}
	repo := rel.Repo
	if repo == nil {
		log.Warn("ConvertRC2SB notifier: repo is nil for release ID %d", rel.ID)
		return
	}

	log.Info("ConvertRC2SB notifier: checking qualification for %s tag %s", repo.FullName(), rel.TagName)

	qualifies, err := RepoQualifiesForConversion(ctx, repo)
	if err != nil {
		log.Warn("ConvertRC2SB notifier: error checking qualification for %s: %v", repo.FullName(), err)
		return
	}
	if !qualifies {
		log.Info("ConvertRC2SB notifier: %s does not qualify, skipping", repo.FullName())
		return
	}

	log.Info("ConvertRC2SB notifier: spawning async conversion for %s tag %s", repo.FullName(), rel.TagName)
	shutdownCtx := graceful.GetManager().ShutdownContext()
	go func(ctx context.Context, repo *repo_model.Repository, release *repo_model.Release) {
		select {
		case <-ctx.Done():
			log.Warn("ConvertRC2SB: context canceled for %s tag %s", repo.FullName(), release.TagName)
			return
		default:
			if err := ForRelease(ctx, repo, release); err != nil {
				log.Error("ConvertRC2SB: conversion failed for %s tag %s: %v", repo.FullName(), release.TagName, err)
			}
		}
	}(shutdownCtx, repo, rel)
}
