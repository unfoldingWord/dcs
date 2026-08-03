// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"context"
	"crypto/md5" // #nosec G501 -- the Scripture Burrito spec mandates MD5 for ingredient checksums
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/dcs"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
)

// Checks in this file only run for Scripture Burrito (sb) repos.

// checkSBIngredients verifies every ingredient entry in metadata.json against the actual
// files at this entry's commit. A declared ingredient whose file doesn't exist is an
// Error (FILE-001); size and MD5 mismatches are Warnings (META-015). Per decision D6,
// MD5 checksums are only computed when validating a tag/release, not on every branch push.
func checkSBIngredients(ctx context.Context, dm *repo_model.Door43Metadata) []*repo_model.Door43HealthcheckIssue {
	sbIngredients := getSBMetadataIngredients(dm)
	if len(sbIngredients) == 0 {
		return nil
	}

	gitRepo, commit := openCommit(ctx, dm)
	if commit == nil {
		return nil
	}
	defer gitRepo.Close()

	// map iteration order is random; sort for deterministic issue order
	paths := make([]string, 0, len(sbIngredients))
	for path := range sbIngredients {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Some SB repos omit the conventional "ingredients/" directory from their ingredient
	// keys, reasoning that the containing property is already named "ingredients".
	// Resolution rule: when no key starts with "ingredients/" and the repo has an
	// ingredients/ directory, every key is resolved under it; when any key starts with
	// "ingredients/", all keys are checked as-is from the repo root.
	hasIngredientsDir := false
	if entry, err := commit.GetTreeEntryByPath("ingredients"); err == nil && entry != nil && entry.IsDir() {
		hasIngredientsDir = true
	}
	prefix := resolveSBIngredientPrefix(paths, hasIngredientsDir)

	var issues []*repo_model.Door43HealthcheckIssue
	mismatch := func(path, reason string) {
		issues = append(issues, newIssue(repo_model.IssueCodeSBIngredientMismatch, repo_model.SeverityLevelWarning,
			fmt.Sprintf(repo_model.IssueCodeSBIngredientMismatch.IssueDetailsFormatString(), path, reason),
			fmt.Sprintf(repo_model.IssueCodeSBIngredientMismatch.IssueSuggestionFormatString(), metadataFileLink(dm))))
	}

	for _, path := range paths {
		ingredient := sbIngredients[path]
		if ingredient == nil {
			continue
		}
		lookupPath := prefix + strings.TrimPrefix(path, "./")
		entry, err := commit.GetTreeEntryByPath(lookupPath)
		if err != nil || entry == nil {
			resolutionNote := ""
			if prefix != "" {
				resolutionNote = fmt.Sprintf(" (resolved to **`%s`**)", lookupPath)
			}
			issues = append(issues, newIssue(repo_model.IssueCodeSBIngredientMissing, repo_model.SeverityLevelError,
				fmt.Sprintf(repo_model.IssueCodeSBIngredientMissing.IssueDetailsFormatString(), path, resolutionNote),
				fmt.Sprintf(repo_model.IssueCodeSBIngredientMissing.IssueSuggestionFormatString(), metadataFileLink(dm))))
			continue
		}
		if entry.IsDir() {
			continue // directories have no size or checksum to compare
		}
		if entry.Size() != ingredient.Size {
			mismatch(path, fmt.Sprintf("has a size of %d in metadata.json but the file is %d bytes", ingredient.Size, entry.Size()))
		}
		// MD5 is only computed for tags/releases (D6) — hashing every blob on every
		// branch push is too expensive for the value it adds.
		if md5sum := ingredient.Checksum["md5"]; md5sum != "" && dm.RefType == "tag" {
			actual, err := blobMD5(entry.Blob())
			if err != nil {
				log.Error("checkSBIngredients: hashing %s/%s: %v", dm.Repo.FullName(), path, err)
				continue
			}
			if !strings.EqualFold(md5sum, actual) {
				mismatch(path, "has an MD5 checksum in metadata.json that does not match the file")
			}
		}
	}
	return issues
}

// resolveSBIngredientPrefix returns "ingredients/" when the metadata's ingredient keys
// omit the conventional directory: no key starts with "ingredients/" and the repo has an
// ingredients/ directory. When any key carries the prefix, keys are taken as-is ("").
func resolveSBIngredientPrefix(paths []string, hasIngredientsDir bool) string {
	for _, path := range paths {
		if strings.HasPrefix(strings.TrimPrefix(path, "./"), "ingredients/") {
			return ""
		}
	}
	if hasIngredientsDir {
		return "ingredients/"
	}
	return ""
}

// getSBMetadataIngredients re-parses the ingredients section of the entry's stored SB metadata
func getSBMetadataIngredients(dm *repo_model.Door43Metadata) map[string]*dcs.SB100Ingredient {
	raw, ok := dm.Metadata["ingredients"]
	if !ok {
		return nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	ingredients := map[string]*dcs.SB100Ingredient{}
	if err := json.Unmarshal(buf, &ingredients); err != nil {
		return nil
	}
	return ingredients
}

func blobMD5(blob *git.Blob) (string, error) {
	dataRc, err := blob.DataAsync()
	if err != nil {
		return "", err
	}
	defer dataRc.Close()
	h := md5.New() // #nosec G401 -- the Scripture Burrito spec mandates MD5 for ingredient checksums
	if _, err := io.Copy(h, dataRc); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
