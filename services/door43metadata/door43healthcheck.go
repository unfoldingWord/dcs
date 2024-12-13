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
	"code.gitea.io/gitea/modules/timeutil"
)

// PerformHealthcheck on Door43Metadata
func PerformHealthcheck(ctx context.Context, dm *repo_model.Door43Metadata) (*repo_model.HealthcheckGroupedIssues, error) {
	dm.LoadHealthcheckIssues(ctx)
	dm.LoadRepo(ctx)
	oldHealthcheckIssues := dm.HealthcheckIssues
	for _, issue := range oldHealthcheckIssues {
		issue.Current = false
		if _, err := db.GetEngine(ctx).ID(issue.ID).Cols("current").Update(issue); err != nil {
			return nil, err
		}
	}
	issues := []*repo_model.Door43HealthcheckIssue{}

	// Check if metadata is valid
	if dm.ValidationError != nil {
		item := &repo_model.Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        repo_model.IssueCodeMetadataInvalid,
			SeverityLevel:    repo_model.SeverityLevelError,
			Details:          repo_model.IssueCodeMetadataInvalid.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(repo_model.IssueCodeMetadataInvalid.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dcs.ConvertValidationErrorToString(dm.ValidationError)),
			Current:          true,
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return nil, err
		}
		issues = append(issues, item)
	}

	if dm.Repo.Owner.LowerName != "unfoldingword" && dm.Publisher == "" || strings.HasPrefix(strings.TrimSpace(dm.Publisher), "unfoldingWord") {
		item := &repo_model.Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        repo_model.IssueCodePublisher,
			SeverityLevel:    repo_model.SeverityLevelError,
			Details:          repo_model.IssueCodePublisher.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(repo_model.IssueCodePublisher.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Repo.OwnerName),
			Current:          true,
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return nil, err
		}
		issues = append(issues, item)
	}

	if dm.Repo.Owner.LowerName != "unfoldingword" && dm.Title == "" || strings.HasPrefix(strings.TrimSpace(dm.Title), "unfoldingWord") {
		item := &repo_model.Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        repo_model.IssueCodeTitle,
			SeverityLevel:    repo_model.SeverityLevelError,
			Details:          repo_model.IssueCodeTitle.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(repo_model.IssueCodeTitle.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Title, strings.Join(strings.Fields(dm.Title)[1:], " ")),
			Current:          true,
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return nil, err
		}
		issues = append(issues, item)
	}

	if dm.Abbreviation == "" || dm.Subject != "Bible" && dm.Subject != "Aligned Bible" {
		if subject, ok := dcs.ResourceToSubjectMap[dm.Abbreviation]; !ok || subject != dm.Subject {
			item := &repo_model.Door43HealthcheckIssue{
				Door43MetadataID: dm.ID,
				IssueCode:        repo_model.IssueCodeAbbreviation,
				SeverityLevel:    repo_model.SeverityLevelError,
				Details:          fmt.Sprintf(repo_model.IssueCodeAbbreviation.IssueDetailsFormatString(), dm.Abbreviation, dm.Subject),
				Suggestion:       fmt.Sprintf(repo_model.IssueCodeAbbreviation.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Abbreviation, dm.Subject, dcs.SubjectToResourceMap[dm.Subject]),
				Current:          true,
			}
			if _, err := db.GetEngine(ctx).Insert(item); err != nil {
				return nil, err
			}
			issues = append(issues, item)
		}
	}

	if dm.Language == "en" && !strings.HasPrefix("en_", dm.Repo.Name) {
		item := &repo_model.Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        repo_model.IssueCodeLanguage,
			SeverityLevel:    repo_model.SeverityLevelWarning,
			Details:          repo_model.IssueCodeLanguage.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(repo_model.IssueCodeLanguage.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref),
			Current:          true,
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return nil, err
		}
		issues = append(issues, item)
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
				Current:          true,
			}
			if _, err := db.GetEngine(ctx).Insert(item); err != nil {
				return nil, err
			}
			issues = append(issues, item)
		}
	}

	if dm.Ingredients != nil {
		doneIngredientTitle := false
		for _, ingredient := range dm.Ingredients {
			// Acts, Numbers and Deuteronomy are only in Englisn and not other languages, so using those
			if !doneIngredientTitle && (ingredient.Title == "" || ingredient.Title == "Numbers" || ingredient.Title == "Deuteronomy" || ingredient.Title == "Acts") {
				doneIngredientTitle = true
				item := &repo_model.Door43HealthcheckIssue{
					Door43MetadataID: dm.ID,
					IssueCode:        repo_model.IssueCodeIngredientTitle,
					SeverityLevel:    repo_model.SeverityLevelError,
					Details:          repo_model.IssueCodeIngredientTitle.IssueDetailsFormatString(),
					Suggestion:       fmt.Sprintf(repo_model.IssueCodeIngredientTitle.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Title),
					Current:          true,
				}
				if _, err := db.GetEngine(ctx).Insert(item); err != nil {
					return nil, err
				}
				issues = append(issues, item)
			}

			if !ingredient.Exists {
				item := &repo_model.Door43HealthcheckIssue{
					Door43MetadataID: dm.ID,
					IssueCode:        repo_model.IssueCodeIngredientMissing,
					SeverityLevel:    repo_model.SeverityLevelError,
					Details:          fmt.Sprintf(repo_model.IssueCodeIngredientMissing.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Path),
					Suggestion:       fmt.Sprintf(repo_model.IssueCodeIngredientMissing.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Identifier, ingredient.Path),
					Current:          true,
				}
				if _, err := db.GetEngine(ctx).Insert(item); err != nil {
					return nil, err
				}
				issues = append(issues, item)
			}

			if ingredient.Size == 0 && !ingredient.IsDir {
				item := &repo_model.Door43HealthcheckIssue{
					Door43MetadataID: dm.ID,
					IssueCode:        repo_model.IssueCodeIngredientEmpty,
					SeverityLevel:    repo_model.SeverityLevelError,
					Details:          fmt.Sprintf(repo_model.IssueCodeIngredientEmpty.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Path),
					Suggestion:       fmt.Sprintf(repo_model.IssueCodeIngredientEmpty.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Identifier, ingredient.Path),
					Current:          true,
				}
				if _, err := db.GetEngine(ctx).Insert(item); err != nil {
					return nil, err
				}
				issues = append(issues, item)
			}
		}
	}

	healthcheckGroupedIssues := repo_model.NewHealthcheckGroupedIssues(issues)
	if dm.Ref == dm.Repo.DefaultBranch {
		if healthcheckGroupedIssues.SeverityLevelCount[repo_model.SeverityLevelError] > 0 {
			item := &repo_model.Door43HealthcheckIssue{
				Door43MetadataID: dm.ID,
				IssueCode:        repo_model.IssueCodeReleaseNeeded,
				SeverityLevel:    repo_model.SeverityLevelError,
				Details:          repo_model.IssueCodeReleaseNeeded.IssueDetailsFormatString(),
				Suggestion:       "Fix all the errors above. Then make a release with [gatewayAdmin](https://gateway-admin.netlify.app/).",
				Current:          true,
			}
			healthcheckGroupedIssues.AddIssue(item)
		} else {
			dm.LoadRepo(ctx)
			err := dm.Repo.LoadLatestDMs(ctx)
			if err != nil {
				return nil, err
			}
			var releaseHealthcheckGroupedIssues *repo_model.HealthcheckGroupedIssues
			if dm.Repo.LatestProdDM != nil {
				if dm.Repo.LatestProdDM.HealthchckUnix == 0 {
					var err error
					if releaseHealthcheckGroupedIssues, err = PerformHealthcheck(ctx, dm.Repo.LatestProdDM); err != nil {
						return nil, err
					}
				} else {
					releaseHealthcheckGroupedIssues, err = repo_model.GetHealthcheckGroupedIssues(ctx, dm.Repo.LatestProdDM.ID)
					if err != nil {
						return nil, err
					}
				}
			}

			if healthcheckGroupedIssues.SeverityLevelCount[repo_model.SeverityLevelError] == 0 && releaseHealthcheckGroupedIssues.SeverityLevelCount[repo_model.SeverityLevelError] > 0 {
				item := &repo_model.Door43HealthcheckIssue{
					Door43MetadataID: dm.ID,
					IssueCode:        repo_model.IssueCodeReleaseNeeded,
					SeverityLevel:    repo_model.SeverityLevelError,
					Details:          repo_model.IssueCodeReleaseNeeded.IssueDetailsFormatString(),
					Suggestion:       fmt.Sprintf(repo_model.IssueCodeReleaseNeeded.IssueSuggestionFormatString(), "all", dm.Ref, repo_model.SeverityLevelError.String()),
					Current:          true,
				}
				if _, err := db.GetEngine(ctx).Insert(item); err != nil {
					return nil, err
				}
				healthcheckGroupedIssues.AddIssue(item)
			} else if healthcheckGroupedIssues.SeverityLevelCount[repo_model.SeverityLevelWarning] < releaseHealthcheckGroupedIssues.SeverityLevelCount[repo_model.SeverityLevelWarning] {
				item := &repo_model.Door43HealthcheckIssue{
					Door43MetadataID: dm.ID,
					IssueCode:        repo_model.IssueCodeReleaseNeeded,
					SeverityLevel:    repo_model.SeverityLevelInfo,
					Details:          repo_model.IssueCodeReleaseNeeded.IssueDetailsFormatString(),
					Suggestion:       fmt.Sprintf(repo_model.IssueCodeReleaseNeeded.IssueSuggestionFormatString(), "some or all", dm.Ref, repo_model.SeverityLevelWarning.String()),
					Current:          true,
				}
				if _, err := db.GetEngine(ctx).Insert(item); err != nil {
					return nil, err
				}
				healthcheckGroupedIssues.AddIssue(item)
			}
		}
	}

	dm.HealthchckUnix = timeutil.TimeStampNow()
	if _, err := db.GetEngine(ctx).ID(dm.ID).Cols("healthchck_unix").Update(dm); err != nil {
		return nil, err
	}

	return healthcheckGroupedIssues, nil
}
