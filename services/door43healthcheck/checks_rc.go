// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"context"
	"fmt"
	"slices"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/dcs"
	"gitea.dev/modules/structs"
)

// Checks in this file only run for Resource Container (rc) repos.

func checkPublisher(_ context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if slices.Contains(uwExemptOwners, dm.Repo.Owner.LowerName) {
		return nil
	}
	if dm.Publisher != "" && !strings.HasPrefix(strings.TrimSpace(dm.Publisher), "unfoldingWord") {
		return nil
	}
	return []*repo_model.Door43HealthcheckIssue{newIssue(repo_model.IssueCodePublisher, repo_model.SeverityLevelWarning,
		repo_model.IssueCodePublisher.IssueDetailsFormatString(),
		fmt.Sprintf(repo_model.IssueCodePublisher.IssueSuggestionFormatString(), metadataFileLink(dm), dm.Repo.OwnerName))}
}

func checkIdentifier(_ context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Abbreviation == "" || (dm.Subject != "Bible" && dm.Subject != "Aligned Bible") {
		identifiers, ok := dcs.SubjectToResourceMap[dm.Subject]
		if !ok || !slices.Contains(identifiers, dm.Abbreviation) {
			return []*repo_model.Door43HealthcheckIssue{newIssue(repo_model.IssueCodeIdentifier, repo_model.SeverityLevelError,
				fmt.Sprintf(repo_model.IssueCodeIdentifier.IssueDetailsFormatString(), dm.Abbreviation, dm.Subject),
				fmt.Sprintf(repo_model.IssueCodeIdentifier.IssueSuggestionFormatString(), metadataFileLink(dm), dm.Abbreviation, dm.Subject, strings.Join(identifiers, ", ")))}
		}
	}
	return nil
}

// checkRelationLanguages warns when a relation's language differs from the resource's
// language (REL-004). Relations are advisory — nothing depends on them being accurate —
// so this is a Warning, not an Error. hbo and el-x-koine are exempt: they are the
// expected languages of the Hebrew/Greek original-text anchors.
func checkRelationLanguages(_ context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	var issues []*repo_model.Door43HealthcheckIssue
	for _, relation := range dm.Relations {
		if relation.Language != dm.Language && relation.Language != "hbo" && relation.Language != "el-x-koine" && dm.Repo.Owner.LowerName != "unfoldingword" {
			issues = append(issues, newIssue(repo_model.IssueCodeRelation, repo_model.SeverityLevelWarning,
				fmt.Sprintf(repo_model.IssueCodeRelation.IssueDetailsFormatString(), relation.FullRelation, dm.Language),
				fmt.Sprintf(repo_model.IssueCodeRelation.IssueSuggestionFormatString(), metadataFileLink(dm), relation.FullRelation, dm.Language, relation.Identifier)))
		}
	}
	return issues
}

func checkTNRelations(_ context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
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
		return []*repo_model.Door43HealthcheckIssue{newIssue(repo_model.IssueCodeRelationMissing, repo_model.SeverityLevelWarning,
			fmt.Sprintf(repo_model.IssueCodeRelationMissing.IssueDetailsFormatString(), strings.Join(missingRelationIdentifiers, ", ")),
			fmt.Sprintf(repo_model.IssueCodeRelationMissing.IssueSuggestionFormatString(), metadataFileLink(dm), strings.Join(missingRelationIdentifiers, ", ")))}
	}
	return nil
}

// relationFallbackOwners are tried when a relation doesn't resolve under the repo's own
// owner (REL-001). Resolution under one of these downgrades the finding to Info.
var relationFallbackOwners = []string{"unfoldingWord", "Door43-Catalog"}

// checkRelationCatalog verifies each relation resolves to a catalog entry — under the
// same owner first, then the fallback owners — and, for Bible/TSV subjects, that this
// resource's books exist in the relation's ingredients. Relations are advisory, so an
// unresolvable relation is a Warning (REL-001/REL-002) and one resolvable only under a
// fallback owner is Info. It queries the door43_metadata table directly rather than
// calling the catalog API over HTTP.
func checkRelationCatalog(ctx context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	if dm.Relations == nil {
		return nil
	}
	var issues []*repo_model.Door43HealthcheckIssue
	for _, relation := range dm.Relations {
		if relation.Language == "" || relation.Identifier == "" {
			continue
		}
		repoName := fmt.Sprintf("%s_%s", relation.Language, relation.Identifier)

		owners := append([]string{dm.Repo.Owner.Name}, relationFallbackOwners...)
		isOrigLanguage := (relation.Language == "hbo" && relation.Identifier == "uhb") || (relation.Language == "el-x-koine" && relation.Identifier == "ugnt")
		if isOrigLanguage {
			// the original-text anchors canonically live under unfoldingWord
			owners = []string{"unfoldingWord"}
		}

		var relDM *repo_model.Door43Metadata
		var resolvedOwner string
		for _, owner := range owners {
			if relDM = getRelationCatalogEntry(ctx, owner, repoName, relation.Version); relDM != nil {
				resolvedOwner = owner
				break
			}
		}

		if relDM == nil {
			issue := newIssue(repo_model.IssueCodeRelationMissing, repo_model.SeverityLevelWarning,
				fmt.Sprintf("Relation %s does not resolve to any catalog entry (looked for repo **`%s`** under **`%s`**)", relation.FullRelation, repoName, strings.Join(owners, "`**, **`")),
				fmt.Sprintf("Verify that the relation %s exists and is properly cataloged, or remove it from the **`relation`** field", relation.FullRelation))
			if relation.Version != "" {
				issue.Rule = "REL-002"
			}
			issues = append(issues, issue)
			continue
		}

		if !isOrigLanguage && !strings.EqualFold(resolvedOwner, dm.Repo.Owner.Name) {
			issues = append(issues, newIssue(repo_model.IssueCodeRelationMissing, repo_model.SeverityLevelInfo,
				fmt.Sprintf("Relation %s resolves under owner **`%s`**, not this repo's owner **`%s`**", relation.FullRelation, resolvedOwner, dm.Repo.Owner.Name),
				fmt.Sprintf("If %s/%s is intended, no action is needed; otherwise create **`%s`** under **`%s`**", resolvedOwner, repoName, repoName, dm.Repo.Owner.Name)))
		}

		// Check if each of this resource's books exists in the relation's catalog entry
		if dm.Ingredients != nil && subjectHasBookIngredients(dm.Subject) && subjectHasBookIngredients(relDM.Subject) {
			for _, ingredient := range dm.Ingredients {
				if ingredient.Identifier == "frt" || ingredient.Identifier == "bak" {
					continue
				}
				found := slices.ContainsFunc(relDM.Ingredients, func(catalogIngredient *structs.Ingredient) bool {
					return catalogIngredient.Identifier == ingredient.Identifier
				})
				if !found {
					issues = append(issues, newIssue(repo_model.IssueCodeRelationMissing, repo_model.SeverityLevelWarning,
						fmt.Sprintf("Relation %s does not contain book %s", relation.FullRelation, ingredient.Identifier),
						fmt.Sprintf("Ensure that the relation %s includes the book %s", relation.FullRelation, ingredient.Identifier)))
				}
			}
		}
	}
	return issues
}

// subjectHasBookIngredients returns true for subjects whose ingredients are Bible books
func subjectHasBookIngredients(subject string) bool {
	return (strings.HasPrefix(subject, "TSV ") || strings.Contains(subject, "Bible")) && !strings.Contains(subject, "OBS")
}

// getRelationCatalogEntry finds the door43_metadata entry the catalog API would serve for
// owner/repoName at the relation's version ("v" prefix optional), or the repo's default
// branch when the relation has no version.
func getRelationCatalogEntry(ctx context.Context, owner, repoName, version string) *repo_model.Door43Metadata {
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, owner, repoName)
	if err != nil {
		return nil
	}
	var refs []string
	if version == "" {
		refs = []string{repo.DefaultBranch}
	} else {
		refs = []string{version, "v" + version}
	}
	for _, ref := range refs {
		if dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref); err == nil && dm != nil {
			return dm
		}
	}
	return nil
}
