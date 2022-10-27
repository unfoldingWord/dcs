// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
//
// Allow "encoding/json" import

/*** DCS Customizations - Module for scrubbing repos ***/

package scrubber

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"reflect"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	repo_service "code.gitea.io/gitea/services/repository"
)

var jsonFilesToScrub = [...]string{
	"project.json",
	"package.json",
	"manifest.json",
	"status.json",
}

var jsonFieldsToScrub = [...]string{
	"translators",
	"contributors",
	"checking_entity",
}

// ScrubSensitiveDataOptions options for scrubbing sensitive data
type ScrubSensitiveDataOptions struct {
	LastCommitID  string
	CommitMessage string
}

// ScrubSensitiveData removes names and email addresses from the manifest|project|package|status.json files and scrubs previous history.
func ScrubSensitiveData(ctx *context.Context, repo *repo.Repository, doer *user_model.User, opts ScrubSensitiveDataOptions) error {
	localPath, err := repo_module.CreateTemporaryPath("repo-scrubber")
	if err != nil {
		return err
	}
	defer func() {
		if err := repo_module.RemoveTemporaryPath(localPath); err != nil {
			log.Error("ScrubSensitiveData: RemoveTemporaryPath: %s", err)
		}
	}()

	if err := git.Clone(db.DefaultContext, repo.RepoPath(), localPath, git.CloneRepoOptions{}); err != nil {
		log.Error("Failed to clone repository: %s (%v)", repo.FullName(), err)
		return fmt.Errorf("failed to clone repository: %s (%v)", repo.FullName(), err)
	}

	if err := ScrubJSONFiles(ctx, localPath); err == nil {
		if err := git.AddChanges(localPath, true); err != nil {
			return fmt.Errorf("AddChanges: %v", err)
		} else if err := git.CommitChanges(localPath, git.CommitChangesOptions{
			Committer: doer.NewGitSig(),
			Message:   opts.CommitMessage,
		}); err != nil {
			return fmt.Errorf("CommitChanges: %v", err)
		} else if err := git.Push(git.DefaultContext, localPath, git.PushOptions{
			Remote: "origin",
			Branch: "master",
			Force:  true,
		}); err != nil {
			return fmt.Errorf("PushForce: %v", err)
		}
		gitRepo, err := git.OpenRepository(*ctx, repo.RepoPath())
		if err != nil {
			return fmt.Errorf("OpenRepository: %v", err)
		}
		commit, err := gitRepo.GetBranchCommit("master")
		if err != nil {
			return fmt.Errorf("GetBranchCommit [branch: %s]: %v", "master", err)
		}
		oldCommitID := opts.LastCommitID
		if err := repo_service.PushUpdate(
			&repo_module.PushUpdateOptions{
				PusherID:     doer.ID,
				PusherName:   doer.Name,
				RepoUserName: repo.MustOwner().Name,
				RepoName:     repo.Name,
				RefFullName:  git.BranchPrefix + "master",
				OldCommitID:  oldCommitID,
				NewCommitID:  commit.ID.String(),
			}); err != nil {
			return fmt.Errorf("PushCommits: %v", err)
		}
	} else {
		return err
	}

	return ScrubCommitNameAndEmail(ctx, localPath, "Door43", "commit@door43.org")
}

// ScrubJSONFiles will scrub all JSON files
func ScrubJSONFiles(ctx *context.Context, localPath string) error {
	for _, fileName := range jsonFilesToScrub {
		if err := scrubJSONFile(ctx, localPath, fileName); err != nil {
			return err
		}
	}
	return nil
}

func scrubJSONFile(ctx *context.Context, localPath, fileName string) error {
	jsonPath := path.Join(localPath, fileName)

	var jsonData interface{}
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return nil // path does not exist, nothing to scrub!
	} else if fileContent, err := os.ReadFile(jsonPath); err != nil {
		log.Error("%v", err)
		return err // error reading file
	} else if err = json.Unmarshal(fileContent, &jsonData); err != nil {
		log.Error("%v", err)
		return err // error unmarhalling file
	}

	m := jsonData.(map[string]interface{})
	ScrubMap(m)

	if fileContent, err := json.MarshalIndent(m, "", "  "); err != nil {
		return err
	} else if err := ScrubFile(ctx, localPath, fileName); err != nil {
		return err
	} else if err := os.WriteFile(jsonPath, fileContent, 0o666); err != nil {
		return err
	}

	return nil
}

// ScrubMap will scrub a map
func ScrubMap(m map[string]interface{}) {
	for _, field := range jsonFieldsToScrub {
		if _, ok := m[field]; ok {
			m[field] = []string{}
		}
	}
	for _, v := range m {
		if reflect.ValueOf(v).Kind() == reflect.Map {
			vm := v.(map[string]interface{})
			ScrubMap(vm)
		}
	}
}

// ScrubFile completely removes a file from a repository's history
func ScrubFile(ctx *context.Context, repoPath, fileName string) error {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	indexFilter := "\"" + gitPath + "\" rm --cached --ignore-unmatch \"" + fileName + "\""
	err = git.NewCommand(*ctx, "filter-branch", "--force", "--prune-empty", "--tag-name-filter", "cat",
		"--index-filter").AddDynamicArguments(indexFilter, "--", "--all").Run(&git.RunOpts{Dir: repoPath})
	if err != nil && err.Error() == "exit status 1" {
		err := os.RemoveAll(path.Join(repoPath, ".git/refs/original/"))
		if err != nil {
			return err
		}
		err = git.NewCommand(*ctx, "reflog", "expire", "--all").Run(&git.RunOpts{Dir: repoPath})
		if err != nil && err.Error() == "exit status 1" {
			return git.NewCommand(*ctx, "gc", "--aggressive", "--prune").Run(&git.RunOpts{Dir: repoPath})
		}
	}
	return err
}

// ScrubCommitNameAndEmail scrubs all commit names and emails
func ScrubCommitNameAndEmail(ctx *context.Context, localPath, newName, newEmail string) error {
	if err := os.RemoveAll(path.Join(localPath, ".git/refs/original/")); err != nil {
		return err
	}
	envFilter := `
export GIT_COMMITTER_NAME="` + newName + `"
export GIT_COMMITTER_EMAIL="` + newEmail + `"
export GIT_AUTHOR_NAME="` + newName + `"
export GIT_AUTHOR_EMAIL="` + newEmail + `"
`
	if err := git.NewCommand(db.DefaultContext, "filter-branch", "-f", "--env-filter").AddDynamicArguments(envFilter, "--tag-name-filter", "cat", "--", "--branches", "--tags").Run(&git.RunOpts{Dir: localPath}); err != nil {
		return err
	}
	return git.NewCommand(*ctx, "push", "--force", "--tags", "origin", "refs/heads/*").Run(&git.RunOpts{Dir: localPath})
}

/*** END DCS Customizations ***/
