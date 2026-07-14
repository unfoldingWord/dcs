// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"

	"gitea.dev/models/door43metadata"
	user_model "gitea.dev/models/user"
)

// GetRepoLanguages gets the languages of the user's repos and returns alphabetized list
func GetRepoLanguages(ctx context.Context, u *user_model.User) []string {
	fields, _ := SearchDoor43MetadataField(ctx, &door43metadata.SearchCatalogOptions{
		Owners:         []string{u.LowerName},
		IsRepoMetadata: true,
		Stage:          door43metadata.StageOther,
	}, "language")
	return fields
}

// GetRepoSubjects gets the subjects of the user's repos and returns alphabetized list
func GetRepoSubjects(ctx context.Context, u *user_model.User) []string {
	fields, _ := SearchDoor43MetadataField(ctx, &door43metadata.SearchCatalogOptions{
		Owners:         []string{u.LowerName},
		IsRepoMetadata: true,
		Stage:          door43metadata.StageOther,
	}, "subject")
	return fields
}

// GetRepoMetadataTypes gets the metadata types of the user's repos and returns alphabetized list
func GetRepoMetadataTypes(ctx context.Context, u *user_model.User) []string {
	fields, _ := SearchDoor43MetadataField(ctx, &door43metadata.SearchCatalogOptions{
		Owners:         []string{u.LowerName},
		IsRepoMetadata: true,
		Stage:          door43metadata.StageOther,
	}, "metadata_type")
	return fields
}
