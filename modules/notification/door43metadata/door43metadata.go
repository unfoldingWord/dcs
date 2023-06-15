// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/repository"
	door43metadata_service "code.gitea.io/gitea/services/door43metadata"
)

type metadataNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &metadataNotifier{}

// NewNotifier create a new actionNotifier notifier
func NewNotifier() base.Notifier {
	return &metadataNotifier{}
}

func (m *metadataNotifier) NotifyNewRelease(ctx context.Context, rel *repo_model.Release) {
	if rel != nil && !rel.IsTag {
		if err := door43metadata_service.ProcessDoor43MetadataForRef(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("ProcessDoor43MetadataForRef: %v\n", err)
		}

		if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, false); err != nil {
			log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyUpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	if rel != nil && !rel.IsTag {
		if err := door43metadata_service.ProcessDoor43MetadataForRef(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("ProcessDoor43MetadataForRef: %v\n", err)
		}

		if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, false); err != nil {
			log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyDeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	if err := repo_model.DeleteDoor43MetadataByRepoIDAndRef(ctx, rel.Repo.ID, rel.TagName); err != nil {
		log.Error("DeleteDoor43MetadataByRepoIDAndRef(ctx, %d, %s): %v\n", rel.Repo.ID, rel.TagName, err)
	}

	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, false); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if opts.RefFullName.IsBranch() {
		if err := door43metadata_service.ProcessDoor43MetadataForRef(ctx, repo, strings.TrimPrefix(opts.RefFullName.String(), git.BranchPrefix)); err != nil {
			log.Error("ProcessDoor43MetadataForRef: %v\n", err)
		}

		if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, false); err != nil {
			log.Error("ProcessDoor43MetadataForRef: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyDeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	if _, err := repo_model.DeleteAllDoor43MetadatasByRepoID(ctx, repo.ID); err != nil {
		log.Error("DeleteAllDoor43MetadatasByRepoID: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyMigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepoRefs(ctx, repo); err != nil {
		log.Error("ProcessDoor43MetadataForRepoRefs: %v\n", err, true)
	}

	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, false); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err, true)
	}
}

func (m *metadataNotifier) NotifyTransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, newOwnerName string) {
	// if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, true); err != nil {
	// 	log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	// }

	// if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, false); err != nil {
	// 	log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	// }
}

func (m *metadataNotifier) NotifyForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, true); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyRenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldName string) {
	// if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, true); err != nil {
	// 	log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	// }

	// if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, false); err != nil {
	// 	log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	// }
}

func (m *metadataNotifier) NotifyDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	if refFullName.IsBranch() {
		ref := refFullName.ShortName()
		if err := repo_model.DeleteDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref); err != nil {
			log.Error("DeleteDoor43MetadataByRepoIDAndRef(ctx, %d, %s): %v\n", repo.ID, ref, err)
		}
	}

	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, false); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	}
}
