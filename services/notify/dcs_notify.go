// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package notify

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

// NewTag notifies new tag release to notifiers
func NewTagRelease(ctx context.Context, rel *repo_model.Release) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadPublisher: %v", err)
		return
	}
	for _, notifier := range notifiers {
		notifier.NewTagRelease(ctx, rel)
	}
}

// RepoTopicsChanged notifies all notifiers that a repository's topics have changed.
// repo.Topics must reflect the new topic state before calling.
func RepoTopicsChanged(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.RepoTopicsChanged(ctx, doer, repo)
	}
}
