// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43Metadata

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/repository"
	"fmt"
	"strings"
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
	if err := models.ProcessDoor43MetadataForRepoRelease(rel.Repo, rel); err != nil {
		fmt.Printf("ProcessDoor43MetadataForRepoRelease: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyUpdateRelease(doer *models.User, rel *models.Release) {
	if err := models.ProcessDoor43MetadataForRepoRelease(rel.Repo, rel); err != nil {
		fmt.Printf("ProcessDoor43MetadataForRepoRelease: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyDeleteRelease(doer *models.User, rel *models.Release) {
	if err := models.DeleteDoor43MetadataByRelease(rel); err != nil {
		fmt.Printf("ProcessDoor43MetadataForRepoRelease: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyPushCommits(pusher *models.User, repo *models.Repository, refName, oldCommitID, newCommitID string, commits *repository.PushCommits) {
	if strings.HasPrefix(refName, git.BranchPrefix) && strings.TrimPrefix(refName, git.BranchPrefix) == repo.DefaultBranch {
		if err := models.ProcessDoor43MetadataForRepoRelease(repo, nil); err != nil {
			fmt.Printf("ProcessDoor43MetadataForRepoRelease: %v\n", err)
		}
	}
}
