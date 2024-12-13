// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/dcs"
)

// PerformHealthcheck on Door43Metadata
func PerformHealthcheck(ctx context.Context, dm *repo_model.Door43Metadata) error {
	if _, err := db.GetEngine(ctx).Delete(&repo_model.Door43HealthcheckIssue{Door43MetadataID: dm.ID}); err != nil {
		return err
	}

	// Check if metadata is valid
	if dm.ValidationError != nil {
		item := &repo_model.Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        repo_model.IssueCodeMetadataInvalid,
			SeverityLevel:    repo_model.SeverityLevelError,
			Details:          repo_model.IssueCodeMetadataInvalid.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(repo_model.IssueCodeMetadataInvalid.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dcs.ConvertValidationErrorToString(dm.ValidationError)),
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return err
		}
	}

	if dm.Publisher == "" || strings.HasPrefix(dm.Publisher, "unfoldingWord") {
		item := &repo_model.Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        repo_model.IssueCodePublisher,
			SeverityLevel:    repo_model.SeverityLevelError,
			Details:          repo_model.IssueCodePublisher.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(repo_model.IssueCodePublisher.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Repo.Owner.Name),
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return err
		}
	}

	if dm.Title == "" || strings.HasPrefix(dm.Title, "unfoldingWord") {
		item := &repo_model.Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        repo_model.IssueCodeTitle,
			SeverityLevel:    repo_model.SeverityLevelError,
			Details:          repo_model.IssueCodeTitle.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(repo_model.IssueCodeTitle.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref),
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return err
		}
	}

	if dm.Abbreviation == "" || dm.Subject != "Bible" && dm.Subject != "Aligned Bible" {
		if subject, ok := dcs.ResourceToSubjectMap[dm.Abbreviation]; !ok || subject != dm.Subject {
			item := &repo_model.Door43HealthcheckIssue{
				Door43MetadataID: dm.ID,
				IssueCode:        repo_model.IssueCodeAbbreviation,
				SeverityLevel:    repo_model.SeverityLevelError,
				Details:          fmt.Sprintf(repo_model.IssueCodeAbbreviation.IssueDetailsFormatString(), dm.Abbreviation, dm.Subject),
				Suggestion:       fmt.Sprintf(repo_model.IssueCodeAbbreviation.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Abbreviation, dm.Subject, dcs.SubjectToResourceMap[dm.Subject]),
			}
			if _, err := db.GetEngine(ctx).Insert(item); err != nil {
				return err
			}
		}
	}

	// Check for relations in other languages
	for _, relation := range dm.Relations {
		if relation.Language != dm.Language && relation.Language != "hbo" && relation.Language != "el-x-koine" {
			item := &repo_model.Door43HealthcheckIssue{
				Door43MetadataID: dm.ID,
				IssueCode:        repo_model.IssueCodeRelation,
				SeverityLevel:    repo_model.SeverityLevelError,
				Details:          fmt.Sprintf(repo_model.IssueCodeRelation.IssueDetailsFormatString(), relation.FullRelation, dm.Language),
				Suggestion:       fmt.Sprintf(repo_model.IssueCodeRelation.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, relation.FullRelation, dm.Language, relation.Identifier),
			}
			if _, err := db.GetEngine(ctx).Insert(item); err != nil {
				return err
			}
		}
	}

	return nil
}
