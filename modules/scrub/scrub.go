package scrub

import (
	"path"
	"io/ioutil"
	"encoding/json"
	"code.gitea.io/git"
	"reflect"
)

var JSON_FILES_TO_SCRUB = [...]string{
	"project.json",
	"package.json",
	"manifest.json",
	"status.json",
}

func ScrubJsonFiles(localPath string) bool {
	success := false
	for _, fileName := range JSON_FILES_TO_SCRUB {
		success = ScrubJsonFile(localPath, fileName) || success
	}

	return success
}

func ScrubJsonFile(localPath, fileName string) bool {
	jsonPath := path.Join(localPath, fileName)

	var jsonData interface{}
	if fileContent, err := ioutil.ReadFile(jsonPath); err != nil {
		return false
	} else {
		if err = json.Unmarshal(fileContent, &jsonData); err != nil {
			return false
		}
	}

	m := jsonData.(map[string]interface{})
	ScrubMap(m)

	if fileContent, err := json.MarshalIndent(m, "", "  "); err != nil {
		return false
	} else {
		if err := git.ScrubFile(localPath, fileName); err != nil {
			return false
		}
		if err := ioutil.WriteFile(jsonPath, []byte(fileContent), 0666); err != nil {
			return false
		}
	}

	return true
}

func ScrubMap(m map[string]interface{}) bool {
	success := false
	fieldsToScrub := [...]string{"translators", "contributors", "checking_entity"}
	for _, field := range fieldsToScrub {
		if _, ok := m[field]; ok {
			m[field] = map[string]interface{}{}
			success = true
		}
	}

	for _, v := range m {
		if reflect.ValueOf(v).Kind() == reflect.Map {
			vm := v.(map[string]interface{})
			success = ScrubMap(vm) || success
		}
	}

	return success
}
