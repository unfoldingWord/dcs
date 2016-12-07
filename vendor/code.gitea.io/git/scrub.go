package git

import (
	"os"
	"path"
	"os/exec"
)

// ScrubFile completely removes a file from a repository's history
func ScrubFile(repoPath string, fileName string) error {
	gitPath, _ := exec.LookPath("git")
	cmd := NewCommand("filter-branch", "--force", "--prune-empty", "--tag-name-filter", "cat",
		"--index-filter", "\""+gitPath+"\" rm --cached --ignore-unmatch "+fileName,
		"--", "--all")
	_, err := cmd.RunInDir(repoPath);
	if err != nil && err.Error() == "exit status 1" {
		os.RemoveAll(path.Join(repoPath, ".git/refs/original/"))
		cmd = NewCommand("reflog", "expire", "--all")
		_, err = cmd.RunInDir(repoPath);
		if err != nil && err.Error() == "exit status 1" {
			cmd = NewCommand("gc", "--aggressive", "--prune")
			_, err = cmd.RunInDir(repoPath);
			return err
		}
	}
	return err
}

// ScrubRepo changes all names and emails in history
func ScrubCommitNameAndEmail(repoPath, newName, newEmail string) error {
	os.RemoveAll(path.Join(repoPath, ".git/refs/original/"))
	if _, err := NewCommand("filter-branch", "-f", "--env-filter", `
export GIT_COMMITTER_NAME="`+newName+`"
export GIT_COMMITTER_EMAIL="`+newEmail+`"
export GIT_AUTHOR_NAME="`+newName+`"
export GIT_AUTHOR_EMAIL="`+newEmail+`"
`, "--tag-name-filter", "cat", "--", "--branches", "--tags").RunInDir(repoPath); err != nil {
		return err
	}
	if _, err := NewCommand("push", "--force", "--tags", "origin", "refs/heads/*").RunInDir(repoPath); err != nil {
		return err
	}
	return nil
}
