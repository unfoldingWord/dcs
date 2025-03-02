// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package notify

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
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
