// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
)

var langNames = map[string]interface{}{}

func contains(strings []string, str string) bool {
	for _, a := range strings {
		if a == str {
			return true
		}
	}
	return false
}

// GetLangNames returns the langnames.json file from tD in a keyed map, loads from tD if not already loaded
func GetLangNames() map[string]interface{} {
	myClient := &http.Client{Timeout: 10 * time.Second}
	if len(langNames) == 0 {
		url := "https://td.unfoldingword.org/exports/langnames.json"
		response, err := myClient.Get(url)
		if err == nil {
			defer response.Body.Close()
			langNamesArr := &[]map[string]interface{}{}
			if err := json.NewDecoder(response.Body).Decode(langNamesArr); err != nil {
				log.Error("Unable to decode langnames.json from tD: %v", err)
			}
			for _, value := range *langNamesArr {
				langNames[value["lc"].(string)] = value
			}
		}
	}
	return langNames
}

// GetRepoLanguages gets the languages of the user's repos and returns alphabetized list
func (u *User) GetRepoLanguages() []string {
	var languages []string
	if repos, _, err := GetUserRepositories(&SearchRepoOptions{Actor: u, Private: false, ListOptions: ListOptions{PageSize: 0}}); err != nil {
		log.Error("Error GetUserRepositories: %v", err)
	} else {
		for _, repo := range repos {
			if dm, err := repo.GetDefaultBranchMetadata(); err != nil {
				log.Error("Error GetDefaultBranchMetadata: %v", err)
			} else if dm != nil {
				lang := (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string)
				if lang != "" && !contains(languages, lang) {
					languages = append(languages, lang)
				}
			} else {
				parts := strings.Split(repo.LowerName, "_")
				if len(parts) > 1 {
					ln := GetLangNames()
					if _, ok := ln[parts[0]]; ok {
						languages = append(languages, parts[0])
					}
				}
			}
		}
	}
	sort.SliceStable(languages, func(i, j int) bool { return strings.ToLower(languages[i]) < strings.ToLower(languages[j]) })
	return languages
}
