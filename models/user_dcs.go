// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"

	"code.gitea.io/gitea/models/door43metadata"
	user_model "code.gitea.io/gitea/models/user"
)

// GetRepoLanguages gets the languages of the user's repos and returns alphabetized list
func GetRepoLanguages(ctx context.Context, u *user_model.User) []string {
	fields, _ := SearchDoor43MetadataField(ctx, &door43metadata.SearchCatalogOptions{
		Owners: []string{u.LowerName},
		Stage:  door43metadata.StageLatest,
	}, "language")
	langs := make([]string, len(fields))
	for i, v := range fields {
		langs[i] = v.(string)
	}
	return langs
}

// GetRepoSubjects gets the subjects of the user's repos and returns alphabetized list
func GetRepoSubjects(ctx context.Context, u *user_model.User) []string {
	fields, _ := SearchDoor43MetadataField(ctx, &door43metadata.SearchCatalogOptions{
		Owners: []string{u.LowerName},
		Stage:  door43metadata.StageLatest,
	}, "subject")
	subs := make([]string, len(fields))
	for i, v := range fields {
		subs[i] = v.(string)
	}
	return subs
}

// GetRepoMetadataTypes gets the metadata types of the user's repos and returns alphabetized list
func GetRepoMetadataTypes(ctx context.Context, u *user_model.User) []string {
	fields, _ := SearchDoor43MetadataField(ctx, &door43metadata.SearchCatalogOptions{
		Owners: []string{u.LowerName},
		Stage:  door43metadata.StageLatest,
	}, "metadata_type")
	types := make([]string, len(fields))
	for i, v := range fields {
		types[i] = v.(string)
	}
	return types
}
