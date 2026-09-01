// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"fmt"
	"slices"

	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	user_model "gitea.dev/models/user"
)

// repoMetadataFacetRow is the scan target of the owner rollup: the three
// door43_metadata columns the user table caches.
type repoMetadataFacetRow struct {
	Language     string
	Subject      string
	MetadataType string
}

// GetRepoMetadataFacets gets the languages, subjects and metadata types of the user's
// repos, each as an alphabetized list of distinct non-empty values.
//
// All three come from one query because they are columns of the same rows. Asking for
// them one column at a time ran three near-identical joins across door43_metadata for
// every owner, and this rollup runs once per owner on every metadata pass.
func GetRepoMetadataFacets(ctx context.Context, u *user_model.User) (languages, subjects, metadataTypes []string, err error) {
	cond := door43metadata.SearchCatalogCondition(&door43metadata.SearchCatalogOptions{
		OwnerIDs:       []int64{u.ID},
		IsRepoMetadata: true,
		Stage:          door43metadata.StageOther,
	})

	var rows []repoMetadataFacetRow
	if err := db.GetEngine(ctx).Table("door43_metadata").
		Distinct("`door43_metadata`.language, `door43_metadata`.subject, `door43_metadata`.metadata_type").
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Where(cond).
		Find(&rows); err != nil {
		return nil, nil, nil, fmt.Errorf("find: %v", err)
	}

	langSet := make(map[string]bool, len(rows))
	subjectSet := make(map[string]bool, len(rows))
	typeSet := make(map[string]bool, len(rows))
	for _, row := range rows {
		langSet[row.Language] = true
		subjectSet[row.Subject] = true
		typeSet[row.MetadataType] = true
	}

	return sortedNonEmptyKeys(langSet), sortedNonEmptyKeys(subjectSet), sortedNonEmptyKeys(typeSet), nil
}

func sortedNonEmptyKeys(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		if value != "" {
			values = append(values, value)
		}
	}
	slices.Sort(values)
	return values
}
