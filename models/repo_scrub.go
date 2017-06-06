package models

import (
	"fmt"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/scrub"

	"github.com/Unknwon/com"
)

// ScrubSensitiveDataOptions options for scrubbing sensitive data
type ScrubSensitiveDataOptions struct {
	LastCommitID  string
	CommitMessage string
}

// ScrubSensitiveData removes names and email addresses from the manifest|project|package|status.json files and scrubs previous history.
func (repo *Repository) ScrubSensitiveData(doer *User, opts ScrubSensitiveDataOptions) error {
	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	localPath := repo.LocalCopyPath()

	if err := repo.DiscardLocalRepoBranchChanges("master"); err != nil {
		return fmt.Errorf("DiscardLocalRepoBranchChanges: %v", err)
	} else if err = repo.UpdateLocalCopyBranch("master"); err != nil {
		return fmt.Errorf("UpdateLocalCopyBranch: %v", err)
	}

	if err := scrub.ScrubJsonFiles(localPath); err == nil {
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

	if err := repo.DiscardLocalRepoBranchChanges("master"); err != nil {
		return fmt.Errorf("DiscardLocalRepoBranchChanges: %v", err)
	} else if err = repo.UpdateLocalCopyBranch("master"); err != nil {
		return fmt.Errorf("UpdateLocalCopyBranch: %v", err)
	}

	if err := scrub.ScrubCommitNameAndEmail(localPath, "Door43", "commit@door43.org"); err != nil {
		return err
	}

	return nil
}
