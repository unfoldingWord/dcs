// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/models/door43metadata"
	user_model "code.gitea.io/gitea/models/user"
)

// GetRepoLanguages gets the languages of the user's repos and returns alphabetized list
func GetRepoLanguages(u *user_model.User) []string {
	fields, _ := SearchDoor43MetadataField(&door43metadata.SearchCatalogOptions{
		Owners: []string{u.LowerName},
		Stage:  door43metadata.StageLatest,
	}, "language")
	return fields
}

// GetRepoSubjects gets the subjects of the user's repos and returns alphabetized list
func GetRepoSubjects(u *user_model.User) []string {
	fields, _ := SearchDoor43MetadataField(&door43metadata.SearchCatalogOptions{
		Owners: []string{u.LowerName},
		Stage:  door43metadata.StageLatest,
	}, "subject")
	return fields
}

// GetRepoMetadataTypes gets the metadata types of the user's repos and returns alphabetized list
func GetRepoMetadataTypes(u *user_model.User) []string {
	fields, _ := SearchDoor43MetadataField(&door43metadata.SearchCatalogOptions{
		Owners: []string{u.LowerName},
		Stage:  door43metadata.StageLatest,
	}, "metadata_type")
	return fields
}
