// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43metadata

import (
	"strings"

	"code.gitea.io/gitea/models"
	template "code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/door43metadata"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/repository"
)

const (
	mailGeneric template.TplName = "notify/generic"
)

type metadataNotifier struct {
	base.NullNotifier
}

var (
	_ base.Notifier = &metadataNotifier{}
)

// NewNotifier create a new metadataNotifier notifier
func NewNotifier() base.Notifier {
	return &metadataNotifier{}
}

func (m *metadataNotifier) NotifyNewRelease(rel *models.Release) {
	if !rel.IsTag {
		if err := door43metadata.ProcessDoor43MetadataForRepoRelease(rel.Repo, rel); err != nil {
			log.Error("ProcessDoor43MetadataForRepoRelease: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyUpdateRelease(doer *models.User, rel *models.Release) {
	if !rel.IsTag {
		if err := door43metadata.ProcessDoor43MetadataForRepoRelease(rel.Repo, rel); err != nil {
			log.Error("ProcessDoor43MetadataForRepoRelease: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyDeleteRelease(doer *models.User, rel *models.Release) {
	if err := models.DeleteDoor43MetadataByRelease(rel); err != nil {
		log.Error("ProcessDoor43MetadataForRepoRelease: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyPushCommits(pusher *models.User, repo *models.Repository, refName, oldCommitID, newCommitID string, commits *repository.PushCommits) {
	if strings.HasPrefix(refName, git.BranchPrefix) && strings.TrimPrefix(refName, git.BranchPrefix) == repo.DefaultBranch {
		if err := door43metadata.ProcessDoor43MetadataForRepoRelease(repo, nil); err != nil {
			log.Info("ProcessDoor43MetadataForRepoRelease: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyDeleteRepository(doer *models.User, repo *models.Repository) {
	if _, err := models.DeleteAllDoor43MetadatasByRepoID(repo.ID); err != nil {
		log.Error("DeleteAllDoor43MetadatasByRepoID: %v\n", err)
	}
}
