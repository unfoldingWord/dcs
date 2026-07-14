// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"

	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/repository"
	notify_service "gitea.dev/services/notify"
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

// processInBackground loads a fresh copy of the repo by ID and processes
// Door43Metadata in a background goroutine. This avoids data races with
// other notifiers that may be reading the same repo pointer concurrently.
func processInBackground(caller string, repo *repo_model.Repository, ref string) {
	repoID := repo.ID
	repoName := repo.FullName()
	shutdownCtx := graceful.GetManager().ShutdownContext()
	go func() {
		select {
		case <-shutdownCtx.Done():
			log.Warn("%s: Context canceled [%s, %s]", caller, repoName, ref)
			return
		default:
			freshRepo, err := repo_model.GetRepositoryByID(shutdownCtx, repoID)
			if err != nil {
				log.Error("%s: GetRepositoryByID(%d) failed: %v", caller, repoID, err)
				return
			}
			if err := ProcessDoor43MetadataForRepo(shutdownCtx, freshRepo, ref); err != nil {
				log.Error("%s: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", caller, repoName, ref, err)
			}
		}
	}()
}

func (m *metadataNotifier) CreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
		log.Error("CreateRepository: ProcessDoor43MetadataForRepo failed [%s]: %v", repo.FullName(), err)
	}
}

func (m *metadataNotifier) NewRelease(ctx context.Context, rel *repo_model.Release) {
	if rel != nil && !rel.IsDraft {
		// Expand files.json / links.json manifest attachments first (can be singular file.json and link.json too)
		// so the metadata processing below sees the final attachments when determining the has_* content flags.
		UnpackJSONAttachments(ctx, rel)

		if err := ProcessDoor43MetadataForRepo(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("NewRelease: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", rel.Repo.FullName(), rel.TagName, err)
		}
	}
}

func (m *metadataNotifier) UpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	if rel != nil && !rel.IsDraft {
		// Expand files.json / links.json manifest attachments first (can be singular file.json and link.json too)
		// so the metadata processing below sees the final attachments when determining the has_* content flags.
		UnpackJSONAttachments(ctx, rel)

		if err := ProcessDoor43MetadataForRepo(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("UpdateRelease: ProcessDoor43MetadataForRepo failed [%s, %s]: %v", rel.Repo.FullName(), rel.TagName, err)
		}
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
		processInBackground("PushCommits", repo, opts.RefFullName.BranchName())
	}
}

func (m *metadataNotifier) SyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if opts.RefFullName.IsBranch() {
		processInBackground("SyncPushCommits", repo, opts.RefFullName.BranchName())
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
	processInBackground("MigrateRepository", repo, "")
}

func (m *metadataNotifier) TransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, newOwnerName string) {
	processInBackground("TransferRepository", repo, "")
}

func (m *metadataNotifier) ForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	processInBackground("ForkRepository", repo, "")
}

func (m *metadataNotifier) RenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldName string) {
	processInBackground("RenameRepository", repo, "")
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
	processInBackground("ChangeDefaultBranch", repo, repo.DefaultBranch)
}
