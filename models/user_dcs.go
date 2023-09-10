// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	user_model "code.gitea.io/gitea/models/user"
)

// GetRepoLanguages gets the languages of the user's repos and returns alphabetized list
func GetRepoLanguages(ctx context.Context, u *user_model.User) []string {
	langs, _ := SearchCatalogLanguages(ctx, &door43metadata.SearchCatalogOptions{
		Owners: []string{u.LowerName},
		Stage:  door43metadata.StageLatest,
		ListOptions: db.ListOptions{
			PageSize: 1000,
		},
	})
	return langs
}

// GetRepoSubjects gets the subjects of the user's repos and returns alphabetized list
func GetRepoSubjects(ctx context.Context, u *user_model.User) []string {
	subjects, _ := SearchCatalogSubjects(ctx, &door43metadata.SearchCatalogOptions{
		Owners: []string{u.LowerName},
		Stage:  door43metadata.StageLatest,
		ListOptions: db.ListOptions{
			PageSize: 1000,
		},
	})
	return subjects
}

// GetRepoMetadataTypes gets the metadata types of the user's repos and returns alphabetized list
func GetRepoMetadataTypes(ctx context.Context, u *user_model.User) []string {
	types, _ := SearchCatalogMetadataTypes(ctx, &door43metadata.SearchCatalogOptions{
		Owners: []string{u.LowerName},
		Stage:  door43metadata.StageLatest,
	})
	return types
}
