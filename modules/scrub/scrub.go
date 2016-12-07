package scrub

import (
	"path"
	"io/ioutil"
	"encoding/json"
	"code.gitea.io/git"
	"reflect"
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
	if fileContent, err := ioutil.ReadFile(jsonPath); err != nil {
		return err
	} else {
		if err = json.Unmarshal(fileContent, &jsonData); err != nil {
			log.Error(3, "%v", err)
			return err
		}
	}

	m := jsonData.(map[string]interface{})
	ScrubMap(m)

	if fileContent, err := json.MarshalIndent(m, "", "  "); err != nil {
		return err
	} else {
		if err := git.ScrubFile(localPath, fileName); err != nil {
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

func ScrubCommitNameAndEmail(localPath, newName, newEmail string) error {
	err := git.ScrubCommitNameAndEmail(localPath, newName, newEmail);
	return err
}
