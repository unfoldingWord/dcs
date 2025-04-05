// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"

	"code.gitea.io/gitea/models/door43metadata"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	notify_service "code.gitea.io/gitea/services/notify"
)

type metadataNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &metadataNotifier{}

func Init(ctx context.Context) error {
	notify_service.RegisterNotifier(NewNotifier())

	return nil
}

// NewNotifier create a new metadataNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &metadataNotifier{}
}

func (m *metadataNotifier) CreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("CreateRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) NewRelease(ctx context.Context, rel *repo_model.Release) {
	if rel != nil && !rel.IsDraft {
		if err := ProcessDoor43MetadataForRepo(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("NewRelease: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", rel.Repo.FullName(), rel.TagName, err)
		}

		// A separate job that handles files.json or links.json files (can be singular file.json and link.json too) as attachments
		UnpackJSONAttachments(ctx, rel)
	}
}

func (m *metadataNotifier) UpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	if rel != nil && !rel.IsDraft {
		if err := ProcessDoor43MetadataForRepo(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("UpdateRelease: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", rel.Repo.FullName(), rel.TagName, err)
		}

		// A separate job that handles files.json or links.json files (can be singular file.json and link.json too) as attachments
		UnpackJSONAttachments(ctx, rel)
	}
}

func (m *metadataNotifier) DeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	// See if the release really was deleted or was just made into a tag
	relDB, err := repo_model.GetReleaseByID(ctx, rel.ID)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		log.Error("GetReleaseByID: %v", err)
	}
	if relDB != nil {
		dm, err := repo_model.GetDoor43MetadataByRepoIDAndReleaseID(ctx, rel.RepoID, rel.ID)
		if err != nil {
			if !repo_model.IsErrDoor43MetadataNotExist(err) {
				log.Error("GetDoor43MetadataByRepoIDAndReleaseID: %v", err)
			}
			return
		}
		dm.Stage = door43metadata.StageOther
		err = repo_model.UpdateDoor43MetadataCols(ctx, dm, "stage")
		if err != nil {
			log.Error("UpdateDoor43MetadataCols: %v", err)
		}

		if err := ProcessDoor43MetadataForRepo(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("DeleteRelease: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", rel.Repo.FullName(), rel.TagName, err)
		}

		return
	}
	err = repo_model.DeleteDoor43MetadataByRepoIDAndReleaseID(ctx, rel.RepoID, rel.ID)
	if err != nil {
		log.Error("DeleteRelease: DeleteDoor43MetadataByRepoIDAndReleaseID failed [repo: %s, releaseID: %d]: %v", rel.Repo.FullName(), rel.ID, err)
	}
}

func (m *metadataNotifier) NewTagRelease(ctx context.Context, rel *repo_model.Release) {
	m.NewRelease(ctx, rel)
}

func (m *metadataNotifier) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if opts.RefFullName.IsBranch() {
		ref := opts.RefFullName.BranchName()
		shutdownCtx := graceful.GetManager().ShutdownContext()
		go func(ctx context.Context, repo *repo_model.Repository, ref string) {
			select {
			case <-ctx.Done():
				log.Warn("PushCommits: Context canceled [%s, %s]", repo.FullName(), ref)
				return
			default:
				if err := ProcessDoor43MetadataForRepo(ctx, repo, ref); err != nil {
					log.Error("PushCommits: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", repo.FullName(), ref, err)
				}
			}
		}(shutdownCtx, repo, ref)
	}
}

func (m *metadataNotifier) SyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if opts.RefFullName.IsBranch() {
		ref := opts.RefFullName.BranchName()
		go func(ctx context.Context, repo *repo_model.Repository, ref string) {
			select {
			case <-ctx.Done():
				log.Warn("SyncPushCommits: Context canceled [%s, %s]", repo.FullName(), ref)
				return
			default:
				if err := ProcessDoor43MetadataForRepo(ctx, repo, ref); err != nil {
					log.Error("SyncPushCommits: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", repo.FullName(), ref, err)
				}
			}
		}(ctx, repo, ref)
	}
}

func (m *metadataNotifier) DeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	if _, err := repo_model.DeleteAllDoor43MetadatasByRepoID(ctx, repo.ID); err != nil {
		log.Error("DeleteRepository: DeleteAllDoor43MetadatasByRepoID failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) SyncDeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	if _, err := repo_model.DeleteAllDoor43MetadatasByRepoID(ctx, repo.ID); err != nil {
		log.Error("SyncDeleteRepository: DeleteAllDoor43MetadatasByRepoID failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) MigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	go func(ctx context.Context, repo *repo_model.Repository) {
		select {
		case <-ctx.Done():
			log.Warn("MigrateRepository: Context canceled [%s]", repo.FullName())
			return
		default:
			if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
				log.Error("MigrateRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
			}
		}
	}(ctx, repo)
}

func (m *metadataNotifier) TransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, newOwnerName string) {
	// Shouldn't really need if the repo is transfered as it keeps the same IDs, releases, etc, but just in case
	go func(ctx context.Context, repo *repo_model.Repository) {
		select {
		case <-ctx.Done():
			log.Warn("TransferRepository: Context canceled [%s]", repo.FullName())
			return
		default:
			if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
				log.Error("TransferRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
			}
		}
	}(ctx, repo)
}

func (m *metadataNotifier) ForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	go func(ctx context.Context, repo *repo_model.Repository) {
		select {
		case <-ctx.Done():
			log.Warn("ForkRepository: Context canceled [%s]", repo.FullName())
			return
		default:
			if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
				log.Error("ForkRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
			}
		}
	}(ctx, repo)
}

func (m *metadataNotifier) RenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldName string) {
	// Shouldn't really need if the repo is renamed as it keeps the same IDs, releases, etc, but just in case
	go func(ctx context.Context, repo *repo_model.Repository) {
		select {
		case <-ctx.Done():
			log.Warn("RenameRepository: Context canceled [%s]", repo.FullName())
			return
		default:
			if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
				log.Error("RenameRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
			}
		}
	}(ctx, repo)
}

func (m *metadataNotifier) DeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	if refFullName.IsBranch() {
		ref := refFullName.ShortName()
		if err := repo_model.DeleteDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref); err != nil {
			log.Error("DeleteRef: DeleteDoor43MetadataByRepoIDAndRef failed [%s, %s]: %v", repo.FullName(), ref, err)
		}
	}
}

func (m *metadataNotifier) ChangeDefaultBranch(ctx context.Context, repo *repo_model.Repository) {
	go func(ctx context.Context, repo *repo_model.Repository) {
		select {
		case <-ctx.Done():
			log.Warn("ChangeDefaultBranch: Context canceled [%s]", repo.FullName())
			return
		default:
			if err := ProcessDoor43MetadataForRepo(ctx, repo, repo.DefaultBranch); err != nil {
				log.Error("ChangeDefaultBranch: ProcessDoor43MetadataForRef failed [%s, %s]: %v", repo.FullName(), repo.DefaultBranch)
			}
		}
	}(ctx, repo)
}
