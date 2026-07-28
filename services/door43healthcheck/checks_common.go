// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"context"
	"fmt"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/dcs"
)

// Checks in this file run for every supported metadata type (rc, ts, tc, sb).

func checkMetadataValid(_ context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.ValidationError == nil {
		return nil
	}
	return []*repo_model.Door43HealthcheckIssue{newIssue(repo_model.IssueCodeMetadataInvalid, repo_model.SeverityLevelError,
		repo_model.IssueCodeMetadataInvalid.IssueDetailsFormatString(),
		fmt.Sprintf(repo_model.IssueCodeMetadataInvalid.IssueSuggestionFormatString(), metadataFileLink(dm), dcs.ConvertValidationErrorToString(dm.ValidationError)))}
}

func checkTitle(_ context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Repo.Owner.LowerName == "unfoldingword" {
		return nil
	}
	if dm.Title != "" && !strings.HasPrefix(strings.TrimSpace(dm.Title), "unfoldingWord") {
		return nil
	}
	fields := strings.Fields(dm.Title)
	var suggestion string
	if len(fields) > 1 {
		suggestion = fmt.Sprintf(repo_model.IssueCodeTitle.IssueSuggestionFormatString(), metadataFileLink(dm), dm.Title, strings.Join(fields[1:], " "))
	} else {
		suggestion = fmt.Sprintf(repo_model.IssueCodeTitle.IssueSuggestionFormatString(), metadataFileLink(dm), dm.Title, dm.Title)
	}
	return []*repo_model.Door43HealthcheckIssue{newIssue(repo_model.IssueCodeTitle, repo_model.SeverityLevelError,
		fmt.Sprintf(repo_model.IssueCodeTitle.IssueDetailsFormatString(), dm.MetadataFileName()),
		suggestion)}
}

func checkLanguage(_ context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Language == "en" && !strings.HasPrefix(dm.Repo.Name, "en_") {
		return []*repo_model.Door43HealthcheckIssue{newIssue(repo_model.IssueCodeLanguage, repo_model.SeverityLevelWarning,
			fmt.Sprintf(repo_model.IssueCodeLanguage.IssueDetailsFormatString(), dm.MetadataFileName()),
			fmt.Sprintf(repo_model.IssueCodeLanguage.IssueSuggestionFormatString(), metadataFileLink(dm)))}
	}
	return nil
}

func checkIngredients(_ context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Ingredients == nil {
		return nil
	}
	var issues []*repo_model.Door43HealthcheckIssue
	doneIngredientTitle := false
	for _, ingredient := range dm.Ingredients {
		// Acts, Numbers and Deuteronomy are only in English and not other languages, so using those
		if !doneIngredientTitle && dm.Repo.Owner.LowerName != "unfoldingword" && (ingredient.Title == "" || (dm.Language != "en" && (ingredient.Title == "Numbers" || ingredient.Title == "Deuteronomy" || ingredient.Title == "Acts"))) {
			doneIngredientTitle = true
			issues = append(issues, newIssue(repo_model.IssueCodeIngredientTitle, repo_model.SeverityLevelError,
				fmt.Sprintf(repo_model.IssueCodeIngredientTitle.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Title),
				fmt.Sprintf(repo_model.IssueCodeIngredientTitle.IssueSuggestionFormatString(), metadataFileLink(dm), ingredient.Title)))
		}

		if !ingredient.Exists {
			issues = append(issues, newIssue(repo_model.IssueCodeIngredientMissing, repo_model.SeverityLevelError,
				fmt.Sprintf(repo_model.IssueCodeIngredientMissing.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Path),
				fmt.Sprintf(repo_model.IssueCodeIngredientMissing.IssueSuggestionFormatString(), metadataFileLink(dm), ingredient.Identifier, ingredient.Path)))
		}

		if ingredient.Exists && ingredient.Size == 0 && !ingredient.IsDir {
			issues = append(issues, newIssue(repo_model.IssueCodeIngredientEmpty, repo_model.SeverityLevelError,
				fmt.Sprintf(repo_model.IssueCodeIngredientEmpty.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Path),
				fmt.Sprintf(repo_model.IssueCodeIngredientEmpty.IssueSuggestionFormatString(), metadataFileLink(dm), ingredient.Identifier, ingredient.Path)))
		}
	}
	return issues
}
