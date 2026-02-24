// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func init() {
	repo_model.HealthcheckFunc = RunHealthcheck
}

// RunHealthcheck performs a full healthcheck on the given Door43Metadata and returns grouped issues.
// It also updates the healthcheck severity and counts in the database if they changed.
func RunHealthcheck(ctx context.Context, dm *repo_model.Door43Metadata) *repo_model.HealthcheckGroupedIssues {
	if dm.MetadataType != "rc" {
		return nil
	}
	dm.LoadRepo(ctx)
	issues := []*repo_model.Door43HealthcheckIssue{}

	issues = append(issues, checkMetadataValid(dm)...)
	issues = append(issues, checkPublisher(dm)...)
	issues = append(issues, checkTitle(dm)...)
	issues = append(issues, checkIdentifier(dm)...)
	issues = append(issues, checkLanguage(dm)...)
	issues = append(issues, checkRelationLanguages(dm)...)
	issues = append(issues, checkIngredients(dm)...)
	issues = append(issues, checkTNRelations(dm)...)
	issues = append(issues, checkRelationCatalog(dm)...)

	// OBS-specific checks
	if dm.Subject == "Open Bible Stories" {
		issues = append(issues, CheckOBSStories(ctx, dm)...)
	}

	healthcheckGroupedIssues := repo_model.NewHealthcheckGroupedIssues(dm.Subject, issues)

	// Check if a release is needed (only for default branch)
	issues = checkReleaseNeeded(ctx, dm, healthcheckGroupedIssues)
	for _, issue := range issues {
		healthcheckGroupedIssues.AddIssue(issue)
	}

	updateHealthcheckInDB(ctx, dm, healthcheckGroupedIssues)

	return healthcheckGroupedIssues
}

func checkMetadataValid(dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.ValidationError == nil {
		return nil
	}
	return []*repo_model.Door43HealthcheckIssue{{
		IssueCode:     repo_model.IssueCodeMetadataInvalid,
		SeverityLevel: repo_model.SeverityLevelError,
		PositiveTitle: repo_model.IssueCodeMetadataInvalid.IssuePositiveString(),
		NegativeTitle: repo_model.IssueCodeMetadataInvalid.IssueNegativeString(),
		Details:       repo_model.IssueCodeMetadataInvalid.IssueDetailsFormatString(),
		Suggestion:    fmt.Sprintf(repo_model.IssueCodeMetadataInvalid.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dcs.ConvertValidationErrorToString(dm.ValidationError)),
	}}
}

func checkPublisher(dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Repo.Owner.LowerName == "unfoldingword" {
		return nil
	}
	if dm.Publisher != "" && !strings.HasPrefix(strings.TrimSpace(dm.Publisher), "unfoldingWord") {
		return nil
	}
	return []*repo_model.Door43HealthcheckIssue{{
		IssueCode:     repo_model.IssueCodePublisher,
		SeverityLevel: repo_model.SeverityLevelError,
		PositiveTitle: repo_model.IssueCodePublisher.IssuePositiveString(),
		NegativeTitle: repo_model.IssueCodePublisher.IssueNegativeString(),
		Details:       repo_model.IssueCodePublisher.IssueDetailsFormatString(),
		Suggestion:    fmt.Sprintf(repo_model.IssueCodePublisher.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Repo.OwnerName),
	}}
}

func checkTitle(dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Repo.Owner.LowerName == "unfoldingword" {
		return nil
	}
	if dm.Title != "" && !strings.HasPrefix(strings.TrimSpace(dm.Title), "unfoldingWord") {
		return nil
	}
	fields := strings.Fields(dm.Title)
	suggestion := dm.Title
	if len(fields) > 1 {
		suggestion = fmt.Sprintf(repo_model.IssueCodeTitle.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Title, strings.Join(fields[1:], " "))
	} else {
		suggestion = fmt.Sprintf(repo_model.IssueCodeTitle.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Title, dm.Title)
	}
	return []*repo_model.Door43HealthcheckIssue{{
		IssueCode:     repo_model.IssueCodeTitle,
		SeverityLevel: repo_model.SeverityLevelError,
		PositiveTitle: repo_model.IssueCodeTitle.IssuePositiveString(),
		NegativeTitle: repo_model.IssueCodeTitle.IssueNegativeString(),
		Details:       repo_model.IssueCodeTitle.IssueDetailsFormatString(),
		Suggestion:    suggestion,
	}}
}

func checkIdentifier(dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Abbreviation == "" || (dm.Subject != "Bible" && dm.Subject != "Aligned Bible") {
		identifiers, ok := dcs.SubjectToResourceMap[dm.Subject]
		if !ok || !slices.Contains(identifiers, dm.Abbreviation) {
			return []*repo_model.Door43HealthcheckIssue{{
				IssueCode:     repo_model.IssueCodeIdentifier,
				SeverityLevel: repo_model.SeverityLevelError,
				PositiveTitle: repo_model.IssueCodeIdentifier.IssuePositiveString(),
				NegativeTitle: repo_model.IssueCodeIdentifier.IssueNegativeString(),
				Details:       fmt.Sprintf(repo_model.IssueCodeIdentifier.IssueDetailsFormatString(), dm.Abbreviation, dm.Subject),
				Suggestion:    fmt.Sprintf(repo_model.IssueCodeIdentifier.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Abbreviation, dm.Subject, strings.Join(identifiers, ", ")),
			}}
		}
	}
	return nil
}

func checkLanguage(dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Language == "en" && !strings.HasPrefix(dm.Repo.Name, "en_") {
		return []*repo_model.Door43HealthcheckIssue{{
			IssueCode:     repo_model.IssueCodeLanguage,
			SeverityLevel: repo_model.SeverityLevelWarning,
			PositiveTitle: repo_model.IssueCodeLanguage.IssuePositiveString(),
			NegativeTitle: repo_model.IssueCodeLanguage.IssueNegativeString(),
			Details:       repo_model.IssueCodeLanguage.IssueDetailsFormatString(),
			Suggestion:    fmt.Sprintf(repo_model.IssueCodeLanguage.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref),
		}}
	}
	return nil
}

func checkRelationLanguages(dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	var issues []*repo_model.Door43HealthcheckIssue
	for _, relation := range dm.Relations {
		if relation.Language != dm.Language && relation.Language != "hbo" && relation.Language != "el-x-koine" && dm.Repo.Owner.LowerName != "unfoldingword" {
			issues = append(issues, &repo_model.Door43HealthcheckIssue{
				IssueCode:     repo_model.IssueCodeRelation,
				SeverityLevel: repo_model.SeverityLevelError,
				PositiveTitle: repo_model.IssueCodeRelation.IssuePositiveString(),
				NegativeTitle: repo_model.IssueCodeRelation.IssueNegativeString(),
				Details:       fmt.Sprintf(repo_model.IssueCodeRelation.IssueDetailsFormatString(), relation.FullRelation, dm.Language),
				Suggestion:    fmt.Sprintf(repo_model.IssueCodeRelation.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, relation.FullRelation, dm.Language, relation.Identifier),
			})
		}
	}
	return issues
}

func checkIngredients(dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Ingredients == nil {
		return nil
	}
	var issues []*repo_model.Door43HealthcheckIssue
	doneIngredientTitle := false
	for _, ingredient := range dm.Ingredients {
		// Acts, Numbers and Deuteronomy are only in English and not other languages, so using those
		if !doneIngredientTitle && dm.Repo.Owner.LowerName != "unfoldingword" && (ingredient.Title == "" || (dm.Language != "en" && (ingredient.Title == "Numbers" || ingredient.Title == "Deuteronomy" || ingredient.Title == "Acts"))) {
			doneIngredientTitle = true
			issues = append(issues, &repo_model.Door43HealthcheckIssue{
				IssueCode:     repo_model.IssueCodeIngredientTitle,
				SeverityLevel: repo_model.SeverityLevelError,
				PositiveTitle: repo_model.IssueCodeIngredientTitle.IssuePositiveString(),
				NegativeTitle: repo_model.IssueCodeIngredientTitle.IssueNegativeString(),
				Details:       fmt.Sprintf(repo_model.IssueCodeIngredientTitle.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Title),
				Suggestion:    fmt.Sprintf(repo_model.IssueCodeIngredientTitle.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Title),
			})
		}

		if !ingredient.Exists {
			issues = append(issues, &repo_model.Door43HealthcheckIssue{
				IssueCode:     repo_model.IssueCodeIngredientMissing,
				SeverityLevel: repo_model.SeverityLevelError,
				PositiveTitle: repo_model.IssueCodeIngredientMissing.IssuePositiveString(),
				NegativeTitle: repo_model.IssueCodeIngredientMissing.IssueNegativeString(),
				Details:       fmt.Sprintf(repo_model.IssueCodeIngredientMissing.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Path),
				Suggestion:    fmt.Sprintf(repo_model.IssueCodeIngredientMissing.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Identifier, ingredient.Path),
			})
		}

		if ingredient.Exists && ingredient.Size == 0 && !ingredient.IsDir {
			issues = append(issues, &repo_model.Door43HealthcheckIssue{
				IssueCode:     repo_model.IssueCodeIngredientEmpty,
				SeverityLevel: repo_model.SeverityLevelError,
				PositiveTitle: repo_model.IssueCodeIngredientEmpty.IssuePositiveString(),
				NegativeTitle: repo_model.IssueCodeIngredientEmpty.IssueNegativeString(),
				Details:       fmt.Sprintf(repo_model.IssueCodeIngredientEmpty.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Path),
				Suggestion:    fmt.Sprintf(repo_model.IssueCodeIngredientEmpty.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Identifier, ingredient.Path),
			})
		}
	}
	return issues
}

func checkTNRelations(dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Subject != "TSV Translation Notes" {
		return nil
	}
	missingRelationIdentifiers := []string{"tw", "ta", "glt", "gst"}
	for _, relation := range dm.Relations {
		id := relation.Identifier
		if id == "ult" {
			id = "glt"
		}
		if id == "ust" {
			id = "gst"
		}
		if slices.Contains(missingRelationIdentifiers, id) {
			missingRelationIdentifiers = repo_model.RemoveStringFromSlice(missingRelationIdentifiers, id)
		}
	}
	if len(missingRelationIdentifiers) > 0 {
		return []*repo_model.Door43HealthcheckIssue{{
			IssueCode:     repo_model.IssueCodeRelationMissing,
			SeverityLevel: repo_model.SeverityLevelWarning,
			PositiveTitle: repo_model.IssueCodeRelationMissing.IssuePositiveString(),
			NegativeTitle: repo_model.IssueCodeRelationMissing.IssueNegativeString(),
			Details:       fmt.Sprintf(repo_model.IssueCodeRelationMissing.IssueDetailsFormatString(), strings.Join(missingRelationIdentifiers, ", ")),
			Suggestion:    fmt.Sprintf(repo_model.IssueCodeRelationMissing.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, strings.Join(missingRelationIdentifiers, ", ")),
		}}
	}
	return nil
}

func checkRelationCatalog(dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Relations == nil {
		return nil
	}
	var issues []*repo_model.Door43HealthcheckIssue
	for _, relation := range dm.Relations {
		ref := relation.Version
		isVersion := false
		if ref == "" {
			ref = dm.Repo.DefaultBranch
		} else {
			isVersion = true
		}
		owner := dm.Repo.Owner.Name
		if (relation.Language == "hbo" && relation.Identifier == "uhb") || (relation.Language == "el-x-koine" && relation.Identifier == "ugnt") {
			owner = "unfoldingWord"
		}
		catalogURL := fmt.Sprintf("%sapi/v1/catalog/entry/%s/%s_%s/%s",
			setting.AppURL, owner, relation.Language, relation.Identifier, ref)
		resp, err := http.Get(catalogURL)
		if err != nil {
			log.Error("Error fetching catalog for relation %s: %v", relation.FullRelation, err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 && isVersion {
			catalogURL = fmt.Sprintf("%sapi/v1/catalog/entry/%s/%s_%s/v%s",
				setting.AppURL, owner, relation.Language, relation.Identifier, ref)
			resp, err = http.Get(catalogURL)
			if err != nil {
				log.Error("Error fetching catalog for relation %s/%s: %v", owner, relation.FullRelation, err)
				continue
			}
			body, err = io.ReadAll(resp.Body)
			resp.Body.Close()
		}

		if resp.StatusCode != 200 {
			issues = append(issues, &repo_model.Door43HealthcheckIssue{
				IssueCode:     repo_model.IssueCodeRelationMissing,
				SeverityLevel: repo_model.SeverityLevelError,
				PositiveTitle: repo_model.IssueCodeRelationMissing.IssuePositiveString(),
				NegativeTitle: repo_model.IssueCodeRelationMissing.IssueNegativeString(),
				Details:       fmt.Sprintf("Relation %s does not exist in the DCS catalog", relation.FullRelation),
				Suggestion:    fmt.Sprintf("Verify that the relation %s exists and is properly cataloged", relation.FullRelation),
			})
			continue
		}

		if err != nil {
			log.Error("Error reading catalog response for relation %s: %v", relation.FullRelation, err)
			continue
		}

		var catalogData struct {
			Subject     string `json:"subject"`
			Ingredients []struct {
				Identifier string `json:"identifier"`
			} `json:"ingredients"`
		}

		if err := json.Unmarshal(body, &catalogData); err != nil {
			log.Error("Error parsing catalog JSON for relation %s: %v", relation.FullRelation, err)
			continue
		}

		// Check if each ingredient exists in the relation's catalog
		if dm.Ingredients != nil && (strings.HasPrefix(dm.Subject, "TSV ") || strings.Contains(dm.Subject, "Bible")) &&
			!strings.Contains(dm.Subject, "OBS") && (strings.HasPrefix(catalogData.Subject, "TSV ") || strings.Contains(catalogData.Subject, "Bible")) &&
			!strings.Contains(catalogData.Subject, "OBS") {
			for _, ingredient := range dm.Ingredients {
				if ingredient.Identifier == "frt" || ingredient.Identifier == "bak" {
					continue
				}
				found := false
				for _, catalogIngredient := range catalogData.Ingredients {
					if ingredient.Identifier == catalogIngredient.Identifier {
						found = true
						break
					}
				}

				if !found {
					issues = append(issues, &repo_model.Door43HealthcheckIssue{
						IssueCode:     repo_model.IssueCodeRelationMissing,
						SeverityLevel: repo_model.SeverityLevelWarning,
						PositiveTitle: repo_model.IssueCodeRelationMissing.IssuePositiveString(),
						NegativeTitle: repo_model.IssueCodeRelationMissing.IssueNegativeString(),
						Details:       fmt.Sprintf("Relation %s does not contain book %s", relation.FullRelation, ingredient.Identifier),
						Suggestion:    fmt.Sprintf("Ensure that the relation %s includes the book %s", relation.FullRelation, ingredient.Identifier),
					})
				}
			}
		}
	}
	return issues
}

func checkReleaseNeeded(ctx context.Context, dm *repo_model.Door43Metadata, hgi *repo_model.HealthcheckGroupedIssues) []*repo_model.Door43HealthcheckIssue {
	dm.LoadRepo(ctx)
	dm.Repo.LoadLatestDMs(ctx)
	if dm.Ref != dm.Repo.DefaultBranch {
		return nil
	}

	if hgi.SeverityLevelCount[repo_model.SeverityLevelError] > 0 {
		return []*repo_model.Door43HealthcheckIssue{{
			IssueCode:     repo_model.IssueCodeReleaseNeeded,
			SeverityLevel: repo_model.SeverityLevelError,
			PositiveTitle: repo_model.IssueCodeReleaseNeeded.IssuePositiveString(),
			NegativeTitle: repo_model.IssueCodeReleaseNeeded.IssueNegativeString(),
			Details:       repo_model.IssueCodeReleaseNeeded.IssueDetailsFormatString(),
			Suggestion:    "Fix all the errors above. Then make a release with <a href=\"https://gateway-admin.netlify.app/\" target=\"_blank\">gatewayAdmin</a>.",
		}}
	}

	if dm.Repo.LatestProdDM == nil {
		return []*repo_model.Door43HealthcheckIssue{{
			IssueCode:     repo_model.IssueCodeReleaseNeeded,
			SeverityLevel: repo_model.SeverityLevelError,
			PositiveTitle: repo_model.IssueCodeReleaseNeeded.IssuePositiveString(),
			NegativeTitle: repo_model.IssueCodeReleaseNeeded.IssueNegativeString(),
			Details:       repo_model.IssueCodeReleaseNeeded.IssueDetailsFormatString(),
			Suggestion:    "No release exists for this resource. Make a release with <a href=\"https://gateway-admin.netlify.app/\" target=\"_blank\">gatewayAdmin</a>.",
		}}
	}

	releaseHGI := RunHealthcheck(ctx, dm.Repo.LatestProdDM)

	if releaseHGI != nil && releaseHGI.SeverityLevelCount[repo_model.SeverityLevelError] > 0 {
		return []*repo_model.Door43HealthcheckIssue{{
			IssueCode:     repo_model.IssueCodeReleaseNeeded,
			SeverityLevel: repo_model.SeverityLevelError,
			PositiveTitle: repo_model.IssueCodeReleaseNeeded.IssuePositiveString(),
			NegativeTitle: repo_model.IssueCodeReleaseNeeded.IssueNegativeString(),
			Details:       repo_model.IssueCodeReleaseNeeded.IssueDetailsFormatString(),
			Suggestion:    fmt.Sprintf(repo_model.IssueCodeReleaseNeeded.IssueSuggestionFormatString(), "all", dm.Ref, repo_model.SeverityLevelError.String()),
		}}
	}

	if releaseHGI != nil && hgi.SeverityLevelCount[repo_model.SeverityLevelWarning] < releaseHGI.SeverityLevelCount[repo_model.SeverityLevelWarning] {
		return []*repo_model.Door43HealthcheckIssue{{
			IssueCode:     repo_model.IssueCodeReleaseNeeded,
			SeverityLevel: repo_model.SeverityLevelInfo,
			PositiveTitle: repo_model.IssueCodeReleaseNeeded.IssuePositiveString(),
			NegativeTitle: repo_model.IssueCodeReleaseNeeded.IssueNegativeString(),
			Details:       repo_model.IssueCodeReleaseNeeded.IssueDetailsFormatString(),
			Suggestion:    fmt.Sprintf(repo_model.IssueCodeReleaseNeeded.IssueSuggestionFormatString(), "some or all", dm.Ref, repo_model.SeverityLevelWarning.String()),
		}}
	}

	return nil
}

func updateHealthcheckInDB(ctx context.Context, dm *repo_model.Door43Metadata, hgi *repo_model.HealthcheckGroupedIssues) {
	if dm.HealthcheckSeverity != hgi.OverallSeverityLevel ||
		dm.HealthcheckCounts == nil ||
		dm.HealthcheckCounts[repo_model.SeverityLevelError] != hgi.SeverityLevelCount[repo_model.SeverityLevelError] ||
		dm.HealthcheckCounts[repo_model.SeverityLevelWarning] != hgi.SeverityLevelCount[repo_model.SeverityLevelWarning] ||
		dm.HealthcheckCounts[repo_model.SeverityLevelInfo] != hgi.SeverityLevelCount[repo_model.SeverityLevelInfo] {
		dm.HealthcheckSeverity = hgi.OverallSeverityLevel
		dm.HealthcheckCounts = hgi.SeverityLevelCount
		if _, err := db.GetEngine(ctx).ID(dm.ID).Cols("healthcheck_severity", "healthcheck_counts").Update(dm); err != nil {
			log.Error("Error updating healthcheck severity and counts: %v", err)
		}
	}
}
