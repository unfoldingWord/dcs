package models

import (
	"fmt"
	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/scrub"
	"github.com/Unknwon/com"
)

// ScrubSensitiveDataOptions options for scrubbing sensitive data
type ScrubSensitiveDataOptions struct {
	LastCommitID  string
	CommitMessage string
}

// ScrubSensitiveData removes names and email addresses from the
// manifest|project|package|status.json files and scrubs previous history.
func (repo *Repository) ScrubSensitiveData(doer *User, opts ScrubSensitiveDataOptions) error {
	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	localPath := repo.LocalCopyPath()

	if err := repo.DiscardLocalRepoBranchChanges("master"); err != nil {
		return fmt.Errorf("DiscardLocalRepoBranchChanges [branch: master]: %v", err)
	} else if err = repo.UpdateLocalCopyBranch("master"); err != nil {
		return fmt.Errorf("UpdateLocalCopyBranch [branch: master]: %v", err)
	}

	if success := scrub.ScrubJsonFiles(localPath); !success {
		return fmt.Errorf("Nothing to scrub")
	}

	if err := git.AddChanges(localPath, true); err != nil {
		return fmt.Errorf("git add --all: %v", err)
	} else if err := git.CommitChanges(localPath, git.CommitChangesOptions{
		Committer: doer.NewGitSig(),
		Message:   opts.CommitMessage,
	}); err != nil {
		return fmt.Errorf("CommitChanges: %v", err)
	} else if err := git.PushForce(localPath, "origin", "master"); err != nil {
		return fmt.Errorf("git push --force --all origin %s: %v", "master", err)
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		log.Error(4, "OpenRepository: %v", err)
		return nil
	}
	commit, err := gitRepo.GetBranchCommit("master")
	if err != nil {
		log.Error(4, "GetBranchCommit [branch: %s]: %v", "master", err)
		return nil
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
		log.Error(4, "CommitRepoAction: %v", err)
		return nil
	}

	return nil
}
