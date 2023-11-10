// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
	"xorm.io/builder"
)

// GetRepoCount returns the total number of repos
func GetRepoCount() int64 {
	sess := db.GetEngine(db.DefaultContext).Table("repository").
		Where(builder.Eq{"is_private": false}).
		And(builder.Eq{"is_archived": false}).
		And(builder.Eq{"is_mirror": false}).
		And(builder.Eq{"is_empty": false})
	count, err := sess.Count()
	if err != nil {
		log.Error("Failed to get number of repos for stats: %v", err)
	}
	return count
}

// GetOrgCount returns the total number of orgs
func GetOrgCount() int64 {
	sess := db.GetEngine(db.DefaultContext).Table("user").
		Where(builder.Eq{"type": user_model.UserTypeOrganization}).
		Where(builder.Eq{"visibility": structs.VisibleTypePublic})
	count, err := sess.Count()
	if err != nil {
		log.Error("Failed to get number of languages for stats: %v", err)
	}
	return count
}

// GetUserCount returns the total number of users
func GetUserCount() int64 {
	sess := db.GetEngine(db.DefaultContext).Table("user").
		Where(builder.Eq{"type": user_model.UserTypeIndividual})
	count, err := sess.Count()
	if err != nil {
		log.Error("Failed to get number of users for stats: %v", err)
	}
	return count
}

// GetCatalogEntryCount returns the total number of top catalog entries
func GetCatalogEntryCount() int64 {
	sess := db.GetEngine(db.DefaultContext).Table("door43_metadata").
		Select("DISTINCT repo_id").
		Where(builder.Eq{"`door43_metadata`.stage": door43metadata.StageProd})
	count, err := sess.Count()
	if err != nil {
		log.Error("Failed to get number of top catalog entries for stats: %v", err)
	}
	return count
}

// GetLanguageCount returns the number of languages with entries in the catalog
func GetLanguageCount() int64 {
	sess := db.GetEngine(db.DefaultContext).Table("door43_metadata").
		Where(builder.Eq{"stage": door43metadata.StageProd}).
		GroupBy("language")
	count, err := sess.Count()
	if err != nil {
		log.Error("Failed to get number of languages for stats: %v", err)
	}
	return count
}

// GetPublisherCount returns the number of orgs with published content in the catalog
func GetPublisherCount() int64 {
	sess := db.GetEngine(db.DefaultContext).Table("door43_metadata").
		Join("INNER", "repository", "`repository`.id = `door43_metadata`.repo_id").
		Where(builder.Eq{"`door43_metadata`.stage": door43metadata.StageProd}).
		GroupBy("`repository`.owner_id")
	count, err := sess.Count()
	if err != nil {
		log.Error("Failed to get number of publishers for stats: %v", err)
	}
	return count
}

// GetActiveProjectCount return the number of repos with a door43metadata release_date_unix in the last 7 days
func GetActiveProjctCount() int64 {
	sess := db.GetEngine(db.DefaultContext).Table("door43_metadata").
		Where("release_date_unix > UNIX_TIMESTAMP(NOW() - INTERVAL 1 WEEK)").
		GroupBy("repo_id")
	count, err := sess.Count()
	if err != nil {
		log.Error("Failed to get number of active projects for stats: %v", err)
	}
	return count
}
