// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43metadata

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/repository"
	door43metadata_service "code.gitea.io/gitea/services/door43metadata"
)

type metadataNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &metadataNotifier{}

// NewNotifier create a new metadataNotifier notifier
func NewNotifier() base.Notifier {
	return &metadataNotifier{}
}

func (m *metadataNotifier) NotifyNewRelease(rel *models.Release) {
	if !rel.IsTag {
		ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyNewRelease rel[%d]%s in [%d]", rel.ID, rel.Title, rel.RepoID))
		defer finished()

		if err := door43metadata_service.ProcessDoor43MetadataForRepoRelease(ctx, rel.Repo, rel); err != nil {
			log.Error("ProcessDoor43MetadataForRepoRelease: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyUpdateRelease(doer *user_model.User, rel *models.Release) {
	if !rel.IsTag {
		ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyUpdateRelease rel[%d]%s in [%d]", rel.ID, rel.Title, rel.RepoID))
		defer finished()

		if err := door43metadata_service.ProcessDoor43MetadataForRepoRelease(ctx, rel.Repo, rel); err != nil {
			log.Error("ProcessDoor43MetadataForRepoRelease: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyDeleteRelease(doer *user_model.User, rel *models.Release) {
	if err := models.DeleteDoor43MetadataByRelease(rel); err != nil {
		log.Error("ProcessDoor43MetadataForRepoRelease: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyPushCommits(pusher *user_model.User, repo *repo.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if strings.HasPrefix(opts.RefFullName, git.BranchPrefix) && strings.TrimPrefix(opts.RefFullName, git.BranchPrefix) == repo.DefaultBranch {
		ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyPushCommits User: %s[%d] in %s[%d]", pusher.Name, pusher.ID, repo.FullName(), repo.ID))
		defer finished()

		if err := door43metadata_service.ProcessDoor43MetadataForRepoRelease(ctx, repo, nil); err != nil {
			log.Info("ProcessDoor43MetadataForRepoRelease: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyDeleteRepository(doer *user_model.User, repo *repo.Repository) {
	if _, err := models.DeleteAllDoor43MetadatasByRepoID(repo.ID); err != nil {
		log.Error("DeleteAllDoor43MetadatasByRepoID: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyMigrateRepository(doer, u *user_model.User, repo *repo.Repository) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(repo); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyTransferRepository(doer *user_model.User, repo *repo.Repository, newOwnerName string) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(repo); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyForkRepository(doer *user_model.User, oldRepo, repo *repo.Repository) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(repo); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyRenameRepository(doer *user_model.User, repo *repo.Repository, oldName string) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(repo); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	}
}
