// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"sort"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/repo"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/log"
)

func contains(strings []string, str string) bool {
	for _, a := range strings {
		if a == str {
			return true
		}
	}
	return false
}

// GetRepoLanguages gets the languages of the user's repos and returns alphabetized list
func GetRepoLanguages(u *user_model.User) []string {
	var languages []string
	if repos, _, err := repo.GetUserRepositories(&repo.SearchRepoOptions{Actor: u, Private: false, ListOptions: db.ListOptions{PageSize: 0}}); err != nil {
		log.Error("Error GetUserRepositories: %v", err)
	} else {
		for _, repo := range repos {
			lang := dcs.GetLanguageFromRepoName(repo.LowerName)
			if lang != "" && !contains(languages, lang) {
				languages = append(languages, lang)
			}
			if dm, err := repo_model.GetDefaultBranchMetadata(repo.ID); err != nil {
				log.Error("Error GetDefaultBranchMetadata: %v", err)
			} else if dm != nil {
				lang = (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string)
				if lang != "" && !contains(languages, lang) {
					languages = append(languages, lang)
				}
			}
		}
	}
	sort.SliceStable(languages, func(i, j int) bool { return strings.ToLower(languages[i]) < strings.ToLower(languages[j]) })
	return languages
}

// GetRepoSubjects gets the subjects of the user's repos and returns alphabetized list
func GetRepoSubjects(u *user_model.User) []string {
	var subjects []string
	if repos, _, err := repo.GetUserRepositories(&repo.SearchRepoOptions{Actor: u, Private: false, ListOptions: db.ListOptions{PageSize: 0}}); err != nil {
		log.Error("Error GetUserRepositories: %v", err)
	} else {
		for _, repo := range repos {
			if dm, err := repo_model.GetDefaultBranchMetadata(repo.ID); err != nil {
				log.Error("Error GetDefaultBranchMetadata: %v", err)
			} else if dm != nil {
				subject := (*dm.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string)
				if subject != "" && !contains(subjects, subject) {
					subjects = append(subjects, subject)
				}
			} else if subject := dcs.GetSubjectFromRepoName(repo.LowerName); subject != "" && !contains(subjects, subject) {
				subjects = append(subjects, subject)
			}
		}
	}
	sort.SliceStable(subjects, func(i, j int) bool { return strings.ToLower(subjects[i]) < strings.ToLower(subjects[j]) })
	return subjects
}
