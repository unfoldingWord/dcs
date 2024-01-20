// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"

	"code.gitea.io/gitea/models/door43metadata"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	notify_service "code.gitea.io/gitea/services/notify"
)

type metadataNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &metadataNotifier{}

func Init() error {
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

func (m *metadataNotifier) SyncCreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("SyncCreateRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
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
	log.Error("WE ARE IN DELETE RELEASE!!!! %s: %s", rel.Repo.FullName(), rel.TagName)
	relDB, err := repo_model.GetReleaseByID(ctx, rel.ID)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		log.Error("GetReleaseByID: %v", err)
	}
	if relDB != nil {
		log.Error("RELDB IS NOT NIL")
		dm, err := repo_model.GetDoor43MetadataByRepoIDAndReleaseID(ctx, rel.RepoID, rel.ID)
		if err != nil {
			if !repo_model.IsErrDoor43MetadataNotExist(err) {
				log.Error("GetDoor43MetadataByRepoIDAndReleaseID: %v", err)
			}
			return
		}
		log.Error("DM IS NOT NIL")
		dm.Stage = door43metadata.StageOther
		err = repo_model.UpdateDoor43MetadataCols(ctx, dm, "stage")
		if err != nil {
			log.Error("UpdateDoor43MetadataCols: %v", err)
		}
		return
	}
	log.Error("DELETING %s %d", rel.TagName, rel.ID)
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
		if err := ProcessDoor43MetadataForRepo(ctx, repo, ref); err != nil {
			log.Error("PushCommits: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", repo.FullName(), ref, err)
		}
	}
}

func (m *metadataNotifier) SyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if opts.RefFullName.IsBranch() {
		ref := opts.RefFullName.BranchName()
		if err := ProcessDoor43MetadataForRepo(ctx, repo, ref); err != nil {
			log.Error("SyncPushCommits: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", repo.FullName(), ref, err)
		}
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
	if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("MigrateRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) TransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, newOwnerName string) {
	// Shouldn't really need if the repo is transfered as it keeps the same IDs, releases, etc, but just in case
	if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("TransferRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) ForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("ForkRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) RenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldName string) {
	// Shouldn't really need if the repo is renamed as it keeps the same IDs, releases, etc, but just in case
	if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("RenameRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) DeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	log.Error("WE ARE IN DELETE REF!!!! %s: %v", repo.FullName(), refFullName)
	if refFullName.IsBranch() {
		ref := refFullName.ShortName()
		if err := repo_model.DeleteDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref); err != nil {
			log.Error("DeleteRef: DeleteDoor43MetadataByRepoIDAndRef failed [%s, %s]: %v", repo.FullName(), ref, err)
		}
	}
}

func (m *metadataNotifier) ChangeDefaultBranch(ctx context.Context, repo *repo_model.Repository) {
	if err := ProcessDoor43MetadataForRepo(ctx, repo, repo.DefaultBranch); err != nil {
		log.Error("ChangeDefaultBranch: ProcessDoor43MetadataForRef failed [%s, %s]: %v", repo.FullName(), repo.DefaultBranch)
		return
	}
}
