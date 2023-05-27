// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43metadata

import (
	"fmt"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
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

func (m *metadataNotifier) NotifyNewRelease(rel *repo_model.Release) {
	if rel != nil && !rel.IsTag {
		ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyNewRelease rel[%d]%s in [%d]", rel.ID, rel.Title, rel.RepoID))
		defer finished()

		if err := door43metadata_service.ProcessDoor43MetadataForRef(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("ProcessDoor43MetadataForRef: %v\n", err)
		}

		if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, false); err != nil {
			log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyUpdateRelease(doer *user_model.User, rel *repo_model.Release) {
	if rel != nil && !rel.IsTag {
		ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyUpdateRelease rel[%d]%s in [%d]", rel.ID, rel.Title, rel.RepoID))
		defer finished()

		if err := door43metadata_service.ProcessDoor43MetadataForRef(ctx, rel.Repo, rel.TagName); err != nil {
			log.Error("ProcessDoor43MetadataForRef: %v\n", err)
		}

		if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, false); err != nil {
			log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyDeleteRelease(doer *user_model.User, rel *repo_model.Release) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyDeleteRelease rel[%d]%s in [%d]", rel.ID, rel.Title, rel.RepoID))
	defer finished()

	if err := repo_model.DeleteDoor43MetadataByRepoIDAndRef(ctx, rel.Repo.ID, rel.TagName); err != nil {
		log.Error("DeleteDoor43MetadataByRepoIDAndRef(ctx, %d, %s): %v\n", rel.Repo.ID, rel.TagName, err)
	}

	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, false); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if strings.HasPrefix(opts.RefFullName, git.BranchPrefix) {
		ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyPushCommits User: %s[%d] in %s[%d]", pusher.Name, pusher.ID, repo.FullName(), repo.ID))
		defer finished()

		if err := door43metadata_service.ProcessDoor43MetadataForRef(ctx, repo, strings.TrimPrefix(opts.RefFullName, git.BranchPrefix)); err != nil {
			log.Error("ProcessDoor43MetadataForRef: %v\n", err)
		}

		if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, false); err != nil {
			log.Error("ProcessDoor43MetadataForRef: %v\n", err)
		}
	}
}

func (m *metadataNotifier) NotifyDeleteRepository(doer *user_model.User, repo *repo_model.Repository) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyDeleteRepository User: %s[%d] in %s[%d]", doer.Name, doer.ID, repo.FullName(), repo.ID))
	defer finished()

	if _, err := repo_model.DeleteAllDoor43MetadatasByRepoID(ctx, repo.ID); err != nil {
		log.Error("DeleteAllDoor43MetadatasByRepoID: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyMigrateRepository(doer, u *user_model.User, repo *repo_model.Repository) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyMigrateRepository User: %s[%d] in %s[%d]", doer.Name, doer.ID, repo.FullName(), repo.ID))
	defer finished()

	if err := door43metadata_service.ProcessDoor43MetadataForRepoRefs(ctx, repo); err != nil {
		log.Error("ProcessDoor43MetadataForRepoRefs: %v\n", err, true)
	}

	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, false); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err, true)
	}
}

func (m *metadataNotifier) NotifyTransferRepository(doer *user_model.User, repo *repo_model.Repository, newOwnerName string) {
	// ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyTransferRepository User: %s[%d] in %s[%d]", doer.Name, doer.ID, repo.FullName(), repo.ID))
	// defer finished()

	// if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, true); err != nil {
	// 	log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	// }

	// if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, false); err != nil {
	// 	log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	// }
}

func (m *metadataNotifier) NotifyForkRepository(doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyForkRepository User: %s[%d] in %s[%d]", doer.Name, doer.ID, repo.FullName(), repo.ID))
	defer finished()

	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, true); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	}
}

func (m *metadataNotifier) NotifyRenameRepository(doer *user_model.User, repo *repo_model.Repository, oldName string) {
	// ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyRenameRepository User: %s[%d] in %s[%d]", doer.Name, doer.ID, repo.FullName(), repo.ID))
	// defer finished()

	// if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, true); err != nil {
	// 	log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	// }

	// if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, rel.Repo, false); err != nil {
	// 	log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	// }
}

func (m *metadataNotifier) NotifyDeleteRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("metadataNotifier.NotifyForkRepository User: %s[%d] in %s[%d]", doer.Name, doer.ID, repo.FullName(), repo.ID))
	defer finished()

	if refType == "branch" {
		ref := strings.TrimPrefix(refFullName, git.BranchPrefix)
		if err := repo_model.DeleteDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref); err != nil {
			log.Error("DeleteDoor43MetadataByRepoIDAndRef(ctx, %d, %s): %v\n", repo.ID, ref, err)
		}
	}

	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, repo, false); err != nil {
		log.Error("ProcessDoor43MetadataForRepo: %v\n", err)
	}
}
