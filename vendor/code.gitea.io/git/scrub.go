package git

import (
	"os"
	"path"
	"fmt"
	"os/exec"
)

// ScrubFile completely removes a file from a repository's history
func ScrubFile(repoPath string, fileName string) error {
	gitPath, _ := exec.LookPath("git")
	cmd := NewCommand("filter-branch", "--force", "--prune-empty", "--tag-name-filter", "cat",
		"--index-filter", "\""+gitPath+"\" rm --cached --ignore-unmatch "+fileName,
		"--", "--all")
	fmt.Println("CMD: ", cmd)
	_, err := cmd.RunInDir(repoPath);
	if err != nil && err.Error() == "exit status 1" {
		os.RemoveAll(path.Join(repoPath, ".git/refs/original/"))
		cmd = NewCommand("reflog", "expire", "--all")
		fmt.Println("CMD: ", cmd)
		_, err = cmd.Run();
		if err != nil && err.Error() == "exit status 1" {
			cmd = NewCommand("gc", "--aggressive", "--prune")
			fmt.Println("CMD: ", cmd)
			_, err = cmd.RunInDir(repoPath);
			return nil
		}
	}
	return err
}

