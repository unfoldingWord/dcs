package scrub

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"reflect"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/log"
)

var JSON_FILES_TO_SCRUB = [...]string{
	"project.json",
	"package.json",
	"manifest.json",
	"status.json",
}

var JSON_FIELDS_TO_SCRUB = [...]string{
	"translators",
	"contributors",
	"checking_entity",
}

func ScrubJsonFiles(localPath string) error {
	for _, fileName := range JSON_FILES_TO_SCRUB {
		if err := ScrubJsonFile(localPath, fileName); err != nil {
			return err
		}
	}
	return nil
}

func ScrubJsonFile(localPath, fileName string) error {
	jsonPath := path.Join(localPath, fileName)

	var jsonData interface{}
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return nil // path does not exist, nothing to scrub!
	} else if fileContent, err := ioutil.ReadFile(jsonPath); err != nil {
		log.Error(3, "%v", err)
		return err // error reading file
	} else {
		if err = json.Unmarshal(fileContent, &jsonData); err != nil {
			log.Error(3, "%v", err)
			return err // error unmarhalling file
		}
	}

	m := jsonData.(map[string]interface{})
	ScrubMap(m)

	if fileContent, err := json.MarshalIndent(m, "", "  "); err != nil {
		return err
	} else {
		if err := ScrubFile(localPath, fileName); err != nil {
			return err
		}
		if err := ioutil.WriteFile(jsonPath, []byte(fileContent), 0666); err != nil {
			return err
		}
	}

	return nil
}

func ScrubMap(m map[string]interface{}) {
	for _, field := range JSON_FIELDS_TO_SCRUB {
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
func ScrubFile(repoPath string, fileName string) error {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	cmd := git.NewCommand("filter-branch", "--force", "--prune-empty", "--tag-name-filter", "cat",
		"--index-filter", "\""+gitPath+"\" rm --cached --ignore-unmatch "+fileName,
		"--", "--all")
	_, err = cmd.RunInDir(repoPath)
	if err != nil && err.Error() == "exit status 1" {
		err := os.RemoveAll(path.Join(repoPath, ".git/refs/original/"))
		if err != nil {
			return err
		}
		cmd = git.NewCommand("reflog", "expire", "--all")
		_, err = cmd.RunInDir(repoPath)
		if err != nil && err.Error() == "exit status 1" {
			cmd = git.NewCommand("gc", "--aggressive", "--prune")
			_, err = cmd.RunInDir(repoPath)
			return err
		}
	}
	return err
}

func ScrubCommitNameAndEmail(localPath, newName, newEmail string) error {
	if err := os.RemoveAll(path.Join(localPath, ".git/refs/original/")); err != nil {
		return err
	}
	if _, err := git.NewCommand("filter-branch", "-f", "--env-filter", `
export GIT_COMMITTER_NAME="`+newName+`"
export GIT_COMMITTER_EMAIL="`+newEmail+`"
export GIT_AUTHOR_NAME="`+newName+`"
export GIT_AUTHOR_EMAIL="`+newEmail+`"
`, "--tag-name-filter", "cat", "--", "--branches", "--tags").RunInDir(localPath); err != nil {
		return err
	}
	if _, err := git.NewCommand("push", "--force", "--tags", "origin", "refs/heads/*").RunInDir(localPath); err != nil {
		return err
	}
	return nil
}
