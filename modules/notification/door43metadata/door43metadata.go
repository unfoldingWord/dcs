// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"

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

func (m *metadataNotifier) NotifyCreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("NotifyCreateRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) NotifySyncCreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("NotifyTransferRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) NotifyNewRelease(ctx context.Context, rel *repo_model.Release) {
	if rel != nil && !rel.IsTag {
		if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("NotifyNewRelease: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", rel.Repo.FullName(), rel.TagName, err)
		}

		// A separate job that handles files.json or links.json files (can be singular file.json and link.json too) as attachments
		door43metadata_service.UnpackJSONAttachments(ctx, rel)
	}
}

func (m *metadataNotifier) NotifyUpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	if rel != nil && !rel.IsTag {
		if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("NotifyUpdateRelease: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", rel.Repo.FullName(), rel.TagName, err)
		}

		// A separate job that handles files.json or links.json files (can be singular file.json and link.json too) as attachments
		door43metadata_service.UnpackJSONAttachments(ctx, rel)
	}
}

func (m *metadataNotifier) NotifyDeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	if err := door43metadata_service.DeleteDoor43MetadataByRepoRef(ctx, rel.Repo, rel.TagName); err != nil {
		log.Error("NotifyDeleteRelease: DeleteDoor43MetadataByRepoRef failed [%s, %s]: %v", rel.Repo.FullName(), rel.TagName, err)
	}
}

func (m *metadataNotifier) NotifyPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if opts.RefFullName.IsBranch() {
		ref := opts.RefFullName.BranchName()
		if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, ref); err != nil {
			log.Error("NotifyPushCommits: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", repo.FullName(), ref, err)
		}
	}
}

func (m *metadataNotifier) NotifySyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if opts.RefFullName.IsBranch() {
		ref := opts.RefFullName.BranchName()
		if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, ref); err != nil {
			log.Error("NotifyPushCommits: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", repo.FullName(), ref, err)
		}
	}
}

func (m *metadataNotifier) NotifyDeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	if _, err := repo_model.DeleteAllDoor43MetadatasByRepoID(ctx, repo.ID); err != nil {
		log.Error("NotifyDeleteRepository: DeleteAllDoor43MetadatasByRepoID failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) NotifySyncDeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	if _, err := repo_model.DeleteAllDoor43MetadatasByRepoID(ctx, repo.ID); err != nil {
		log.Error("NotifyDeleteRepository: DeleteAllDoor43MetadatasByRepoID failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) NotifyMigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("NotifyMigrateRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) NotifyTransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, newOwnerName string) {
	// Shouldn't really need if the repo is transfered as it keeps the same IDs, releases, etc, but just in case
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("NotifyTransferRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) NotifyForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("NotifyForkRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) NotifyRenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldName string) {
	// Shouldn't really need if the repo is renamed as it keeps the same IDs, releases, etc, but just in case
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("NotifyRenameRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) NotifyDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	if refFullName.IsBranch() {
		ref := refFullName.ShortName()
		if err := door43metadata_service.DeleteDoor43MetadataByRepoRef(ctx, repo, ref); err != nil {
			log.Error("NotifyDeleteRef: DeleteDoor43MetadataByRepoRef failed [%s, %s]: %v", repo.FullName(), ref, err)
		}
	}
}
