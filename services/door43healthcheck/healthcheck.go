// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"context"
	"fmt"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
)

func init() {
	repo_model.HealthcheckFunc = RunHealthcheck
}

// checkFunc runs one health check against a Door43Metadata entry
type checkFunc func(ctx context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue

// Supported returns true if health checks can run against the given metadata type
func Supported(metadataType string) bool {
	switch metadataType {
	case "rc", "ts", "tc", "sb":
		return true
	}
	return false
}

// checksFor returns the checks applicable to the entry's metadata type and subject
func checksFor(dm *repo_model.Door43Metadata) []checkFunc {
	checks := []checkFunc{checkMetadataValid, checkTitle, checkLanguage, checkIngredients}
	switch dm.MetadataType {
	case "rc":
		checks = append(checks, checkPublisher, checkIdentifier, checkRepoNameLanguage, checkRelationLanguages, checkTNRelations, checkRelationCatalog)
		switch {
		case dm.Subject == "Open Bible Stories":
			checks = append(checks, CheckOBSStories)
		case repo_model.IsScriptureSubject(dm.Subject):
			checks = append(checks, checkUSFMBooks)
		case repo_model.IsTSVSubject(dm.Subject):
			checks = append(checks, checkTSVFiles)
		}
	case "tc":
		checks = append(checks, checkUSFMBooks)
	case "sb":
		checks = append(checks, checkRepoNameLanguage, checkSBIngredients)
		switch {
		case repo_model.IsScriptureSubject(dm.Subject):
			checks = append(checks, checkUSFMBooks)
		case repo_model.IsTSVSubject(dm.Subject):
			checks = append(checks, checkTSVFiles)
		}
	}
	return checks
}

// RunHealthcheck performs a full healthcheck on the given Door43Metadata and returns grouped issues.
// It also stores the issues and the resulting severity and counts in the database.
func RunHealthcheck(ctx context.Context, dm *repo_model.Door43Metadata) *repo_model.HealthcheckGroupedIssues {
	if dm == nil || !Supported(dm.MetadataType) {
		return nil
	}
	if err := dm.LoadRepo(ctx); err != nil || dm.Repo == nil {
		log.Error("RunHealthcheck: LoadRepo for DM %d: %v", dm.ID, err)
		return nil
	}
	// LoadRepo skips owner loading when the pipeline pre-set dm.Repo, and several
	// checks dereference dm.Repo.Owner — LoadOwner is idempotent, so always ensure it
	if err := dm.Repo.LoadOwner(ctx); err != nil || dm.Repo.Owner == nil {
		log.Error("RunHealthcheck: LoadOwner for DM %d (repo %s): %v", dm.ID, dm.Repo.FullName(), err)
		return nil
	}

	issues := []*repo_model.Door43HealthcheckIssue{}
	if dm.ValidationError != nil {
		// A schema-invalid metadata file (L1 failure) makes every deeper check
		// unreliable — the entry's fields were backfilled, not extracted — so only
		// the invalid-metadata finding is reported (spec §4).
		issues = checkMetadataValid(ctx, dm)
	} else {
		for _, check := range checksFor(dm) {
			issues = append(issues, check(ctx, dm)...)
		}
	}

	hgi := repo_model.NewHealthcheckGroupedIssues(dm.MetadataType, dm.Subject, issues)

	// Release advice runs last so it can compare against the check results above.
	for _, issue := range checkReleaseNeeded(ctx, dm, hgi) {
		hgi.AddIssue(issue)
		issues = append(issues, issue)
	}

	saveHealthcheck(ctx, dm, hgi, issues)

	return hgi
}

// checkReleaseNeeded advises when the default branch is ahead of, or missing, an error-free
// release. Severity is Info only: publishing is never gated on health; the catalog just
// reports each entry's own severity.
func checkReleaseNeeded(ctx context.Context, dm *repo_model.Door43Metadata, hgi *repo_model.HealthcheckGroupedIssues) []*repo_model.Door43HealthcheckIssue {
	if dm.Repo == nil || dm.Ref != dm.Repo.DefaultBranch {
		return nil
	}
	_ = dm.Repo.LoadLatestDMs(ctx)

	releaseNeeded := func(suggestion string) []*repo_model.Door43HealthcheckIssue {
		return []*repo_model.Door43HealthcheckIssue{newIssue(repo_model.IssueCodeReleaseNeeded, repo_model.SeverityLevelInfo,
			repo_model.IssueCodeReleaseNeeded.IssueDetailsFormatString(), suggestion)}
	}

	if hgi.SeverityLevelCount[repo_model.SeverityLevelError] > 0 {
		return releaseNeeded("Fix all the errors above. Then make a release with <a href=\"https://gateway-admin.netlify.app/\" target=\"_blank\">gatewayAdmin</a>.")
	}

	prodDM := dm.Repo.LatestProdDM
	if prodDM == nil {
		return releaseNeeded("No release exists for this resource. Make a release with <a href=\"https://gateway-admin.netlify.app/\" target=\"_blank\">gatewayAdmin</a>.")
	}

	prodCounts := prodDM.HealthcheckCounts
	if prodDM.HealthcheckSeverity == 0 {
		// the release pre-dates per-ref checks and was never checked; check it now so its stored results exist
		if prodHGI := RunHealthcheck(ctx, prodDM); prodHGI != nil {
			prodCounts = prodHGI.SeverityLevelCount
		}
	}

	if prodCounts[repo_model.SeverityLevelError] > 0 {
		return releaseNeeded(fmt.Sprintf(repo_model.IssueCodeReleaseNeeded.IssueSuggestionFormatString(), "all", dm.Ref, repo_model.SeverityLevelError.String()))
	}

	if hgi.SeverityLevelCount[repo_model.SeverityLevelWarning] < prodCounts[repo_model.SeverityLevelWarning] {
		return releaseNeeded(fmt.Sprintf(repo_model.IssueCodeReleaseNeeded.IssueSuggestionFormatString(), "some or all", dm.Ref, repo_model.SeverityLevelWarning.String()))
	}

	return nil
}

// saveHealthcheck stores the issues and the severity/counts for the entry.
// Synthesized entries (ID == 0) have nothing to store against, and an entry whose ref
// was deleted while the check ran is skipped so no results outlive their ref.
func saveHealthcheck(ctx context.Context, dm *repo_model.Door43Metadata, hgi *repo_model.HealthcheckGroupedIssues, issues []*repo_model.Door43HealthcheckIssue) {
	if dm.ID == 0 {
		return
	}
	dm.HealthcheckSeverity = hgi.OverallSeverityLevel
	dm.HealthcheckCounts = hgi.SeverityLevelCount
	dm.HealthcheckTimeUnix = timeutil.TimeStampNow()
	stored, err := repo_model.StoreHealthcheckResults(ctx, dm, issues)
	if err != nil {
		log.Error("saveHealthcheck: storing results for DM %d: %v", dm.ID, err)
	} else if !stored {
		log.Debug("saveHealthcheck: DM %d was deleted while its check ran; results discarded", dm.ID)
	}
}

// newIssue builds an issue with the standard positive/negative titles for its code and
// the code's default rule ID from the DCS Resource Validation Specification. Checks whose
// code covers several spec rules overwrite Rule on the returned issue.
func newIssue(code repo_model.IssueCode, severity repo_model.SeverityLevel, details, suggestion string) *repo_model.Door43HealthcheckIssue {
	return &repo_model.Door43HealthcheckIssue{
		IssueCode:     code,
		Rule:          repo_model.IssueCodeDefaultRules[code],
		SeverityLevel: severity,
		PositiveTitle: code.IssuePositiveString(),
		NegativeTitle: code.IssueNegativeString(),
		Details:       details,
		Suggestion:    suggestion,
	}
}

// metadataFileLink returns an HTML link to the entry's metadata file, for use in suggestions
func metadataFileLink(dm *repo_model.Door43Metadata) string {
	fileName := dm.MetadataFileName()
	return fmt.Sprintf("<a href=\"%s/src/branch/%s/%s\" target=\"_blank\">%s</a>", dm.Repo.Link(), dm.Ref, fileName, fileName)
}

// openCommit opens the entry's git repository at its commit.
// The caller must Close the returned git repo when commit is not nil.
func openCommit(ctx context.Context, dm *repo_model.Door43Metadata) (*git.Repository, *git.Commit) {
	if dm.Repo == nil || dm.CommitSHA == "" {
		return nil, nil
	}
	gitRepo, err := git.OpenRepository(ctx, dm.Repo)
	if err != nil {
		log.Error("openCommit: OpenRepository %s: %v", dm.Repo.FullName(), err)
		return nil, nil
	}
	commit, err := gitRepo.GetCommit(ctx, dm.CommitSHA)
	if err != nil {
		log.Error("openCommit: GetCommit %s/%s: %v", dm.Repo.FullName(), dm.CommitSHA, err)
		gitRepo.Close()
		return nil, nil
	}
	return gitRepo, commit
}
