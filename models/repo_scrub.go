// Copyright 2019 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

/*** DCS Custom Code - Model for repo scrubbing ***/

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/scrubber"
)

// ScrubSensitiveDataOptions options for scrubbing sensitive data
type ScrubSensitiveDataOptions struct {
	LastCommitID  string
	CommitMessage string
}

// ScrubSensitiveData removes names and email addresses from the manifest|project|package|status.json files and scrubs previous history.
func (repo *Repository) ScrubSensitiveData(doer *User, opts ScrubSensitiveDataOptions) error {
	localPath, err := CreateTemporaryPath("repo-scrubber")
	if err != nil {
		return err
	}
	defer func() {
		if err := RemoveTemporaryPath(localPath); err != nil {
			log.Error("ScrubSensitiveData: RemoveTemporaryPath: %s", err)
		}
	}()

	if err := git.Clone(repo.RepoPath(), localPath, git.CloneRepoOptions{}); err != nil {
		log.Error("Failed to clone repository: %s (%v)", repo.FullName(), err)
		return fmt.Errorf("failed to clone repository: %s (%v)", repo.FullName(), err)
	}

	if err := scrubber.ScrubJSONFiles(localPath); err == nil {
		if err := git.AddChanges(localPath, true); err != nil {
			return fmt.Errorf("AddChanges: %v", err)
		} else if err := git.CommitChanges(localPath, git.CommitChangesOptions{
			Committer: doer.NewGitSig(),
			Message:   opts.CommitMessage,
		}); err != nil {
			return fmt.Errorf("CommitChanges: %v", err)
		} else if err := git.Push(localPath, git.PushOptions{
			Remote: "origin",
			Branch: "master",
			Force:  true,
		}); err != nil {
			return fmt.Errorf("PushForce: %v", err)
		}
		gitRepo, err := git.OpenRepository(repo.RepoPath())
		if err != nil {
			return fmt.Errorf("OpenRepository: %v", err)
		}
		commit, err := gitRepo.GetBranchCommit("master")
		if err != nil {
			return fmt.Errorf("GetBranchCommit [branch: %s]: %v", "master", err)
		}
		// Simulate push event.
		pushCommits := &PushCommits{
			Len:     1,
			Commits: []*PushCommit{CommitToPushCommit(commit)},
		}
		oldCommitID := opts.LastCommitID
		if err := CommitRepoAction(CommitRepoActionOptions{
			PusherName:  doer.Name,
			RepoOwnerID: repo.MustOwner().ID,
			RepoName:    repo.Name,
			RefFullName: git.BranchPrefix + "master",
			OldCommitID: oldCommitID,
			NewCommitID: commit.ID.String(),
			Commits:     pushCommits,
		}); err != nil {
			return fmt.Errorf("CommitRepoAction: %v", err)
		}
	} else {
		return err
	}

	return scrubber.ScrubCommitNameAndEmail(localPath, "Door43", "commit@door43.org")
}
