// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/convert"

	"github.com/google/uuid"
	"xorm.io/builder"
)

func processDoor43MetadataForRepoRefs(ctx context.Context, repo *repo_model.Repository) error {
	refs, err := repo_model.GetRepoReleaseTagsForMetadata(ctx, repo.ID)
	if err != nil {
		log.Error("GetRepoReleaseTagsForMetadata Error %s: %v", repo.FullName(), err)
	}

	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		log.Error("git.OpenRepository Error %s: %v", repo.FullName(), err)
	}
	if gitRepo != nil {
		defer gitRepo.Close()
		branchNames, _, err := gitRepo.GetBranchNames(0, 0)
		if err != nil {
			log.Error("git.GetBranchNames Error %s: %v", repo.FullName(), err)
		} else {
			refs = append(refs, branchNames...)
		}
	}

	for _, ref := range refs {
		if err := processDoor43MetadataForRepoRef(ctx, repo, ref); err != nil {
			log.Info("Failed to process metadata for repo %s, ref %s: %v", repo.FullName(), ref, err)
			if err = system.CreateRepositoryNotice("Failed to process metadata for repository (%s) ref (%s): %v", repo.FullName(), ref, err); err != nil {
				log.Error("processDoor43MetadataForRepoRef: %v", err)
			}
		}
	}
	return nil
}

func handleLatestStageDM(ctx context.Context, repo *repo_model.Repository, stage door43metadata.Stage, earliestDate *timeutil.TimeStamp) (*repo_model.Door43Metadata, error) {
	_, err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": repo.ID}).
		And(builder.Eq{"stage": stage}).
		Cols("is_latest_for_stage").
		Update(&repo_model.Door43Metadata{IsLatestForStage: false})
	if err != nil {
		return nil, err
	}

	var dm *repo_model.Door43Metadata
	if stage == door43metadata.StageLatest {
		dm, err = repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, repo.DefaultBranch)
	} else {
		dm, err = repo_model.GetMostRecentDoor43MetadataByStage(ctx, repo.ID, stage)
	}
	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		return nil, err
	}
	if dm != nil && (earliestDate == nil || dm.ReleaseDateUnix > *earliestDate) {
		dm.Stage = stage
		dm.IsLatestForStage = true
		err = repo_model.UpdateDoor43MetadataCols(ctx, dm, "stage", "is_latest_for_stage")
		if err != nil {
			return nil, err
		}
	}

	return dm, nil
}

func handleRepoDM(ctx context.Context, repo *repo_model.Repository) error {
	if repo.DefaultBranchDM != nil {
		repo.RepoDM = repo.DefaultBranchDM
	} else if repo.LatestProdDM != nil {
		repo.RepoDM = repo.LatestProdDM
	} else if repo.LatestPreprodDM != nil {
		repo.RepoDM = repo.LatestPreprodDM
	} else {
		repo.RepoDM, _ = repo_model.GetMostRecentDoor43MetadataByStage(ctx, repo.ID, door43metadata.StageBranch)
	}

	if repo.RepoDM == nil || !repo.RepoDM.IsRepoMetadata {
		_, err := db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repo.ID}).
			Cols("is_repo_metadata").
			Update(&repo_model.Door43Metadata{IsRepoMetadata: false})
		if err != nil {
			log.Error("handleRepoDM: failed to update all Door43Metadatas [%s]: %v", repo.FullName(), err)
		}
	}

	if repo.RepoDM != nil && !repo.RepoDM.IsRepoMetadata {
		repo.RepoDM.IsRepoMetadata = true
		err := repo_model.UpdateDoor43MetadataCols(ctx, repo.RepoDM, "is_repo_metadata")
		if err != nil {
			log.Error("handleRepoDM: failed to update Door43Metadata [%s, %d]: %v", repo.FullName(), repo.RepoDM.ID, err)
		}
	}

	return nil
}

// processDoor43MetadataForRepoLatestDMs determines the latest DMs for a repo
func processDoor43MetadataForRepoLatestDMs(ctx context.Context, repo *repo_model.Repository) error {
	// Handle Stage Latest
	dm, err := handleLatestStageDM(ctx, repo, door43metadata.StageLatest, nil)
	if err != nil {
		log.Error("handleLatestStageDM for default branch [%s, %s]: %v", repo.FullName(), repo.DefaultBranch, err)
	}
	repo.DefaultBranchDM = dm

	// Handle Stage Prod
	dm, err = handleLatestStageDM(ctx, repo, door43metadata.StageProd, nil)
	if err != nil {
		log.Error("handleLatestStageDM for prod [%s]: %v", repo.FullName(), err)
	}
	repo.LatestProdDM = dm

	// Handle Stage Preprod
	var earliestDate *timeutil.TimeStamp
	if repo.LatestProdDM != nil {
		earliestDate = &repo.LatestProdDM.ReleaseDateUnix
	}
	dm, err = handleLatestStageDM(ctx, repo, door43metadata.StagePreProd, earliestDate)
	if err != nil {
		log.Error("handleLatestStageDM for preprod [%s]: %v", repo.FullName(), err)
	}
	repo.LatestPreprodDM = dm

	err = handleRepoDM(ctx, repo)
	if err != nil {
		log.Error("handleRepoDM [%s]: %v", repo.FullName(), err)
	}

	return nil
}

// ProcessDoor43MetadataForRepo handles the metadata for a given repo for all its releases
func ProcessDoor43MetadataForRepo(ctx context.Context, repo *repo_model.Repository, ref string) error {
	if ctx == nil || repo == nil {
		return fmt.Errorf("no repository provided")
	}

	if repo.IsArchived || repo.IsPrivate {
		_, err := repo_model.DeleteAllDoor43MetadatasByRepoID(ctx, repo.ID)
		if err != nil {
			log.Error("DeleteAllDoor43MetadatasByRepoID: %v", err)
		}
		return err // No need to process any thing else below
	}

	if ref == "" {
		log.Debug(">>>>>> PROCESSING REFS: %s", repo.FullName())
		if err := processDoor43MetadataForRepoRefs(ctx, repo); err != nil {
			// log error but keep on going
			log.Error("processDoor43MetadataForRepoRefs %s Error: %v", repo.FullName(), err)
		}
	} else {
		if err := processDoor43MetadataForRepoRef(ctx, repo, ref); err != nil {
			// log error but keep on going
			log.Error("processDoor43MetadataForRepoRefs %s Error: %v", repo.FullName(), err)
		}
	}

	return processDoor43MetadataForRepoLatestDMs(ctx, repo)
}

func GetBookAlignmentCount(bookPath string, commit *git.Commit) (int, error) {
	blob, err := commit.GetBlobByPath(bookPath)
	if err != nil {
		if !git.IsErrNotExist(err) {
			log.Error("GetBlobByPath(%s) Error: %v\n", bookPath, err)
		}
		return 0, err
	}
	dataRc, err := blob.DataAsync()
	if err != nil {
		log.Error("blob.DataAsync() Error: %v\n", err)
		return 0, err
	}
	defer dataRc.Close()

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))
	buf, err = io.ReadAll(rd)
	if err != nil {
		log.Error("io.ReadAll Error: %v", err)
		return 0, err
	}
	matches := regexp.MustCompile(`\\zaln-s`).FindAllStringIndex(string(buf), -1)
	return len(matches), nil
}

// GetBooks get the books of the manifest
func GetBooks(manifest map[string]interface{}) []string {
	var books []string
	if len((manifest)["projects"].([]interface{})) > 0 {
		for _, prod := range (manifest)["projects"].([]interface{}) {
			books = append(books, prod.(map[string]interface{})["identifier"].(string))
		}
	}
	return books
}

func GetDoor43MetadataFromRCManifest(dm *repo_model.Door43Metadata, manifest map[string]interface{}, repo *repo_model.Repository, commit *git.Commit) error {
	var metadataType string
	var metadataVersion string
	var subject string
	var flavorType string
	var flavor string
	var abbreviation string
	var title string
	var language string
	var languageTitle string
	var languageDirection string
	var languageIsGL bool
	var format string
	var contentFormat string
	var checkingLevel int
	var ingredients []*structs.Ingredient

	re := regexp.MustCompile("^([^0-9]+)(.*)$")
	matches := re.FindStringSubmatch(manifest["dublin_core"].(map[string]interface{})["conformsto"].(string))
	if len(matches) == 3 {
		metadataType = matches[1]
		metadataVersion = matches[2]
	} else {
		// should never get here since schema validated
		metadataType = "rc"
		metadataVersion = "0.2"
	}
	subject = manifest["dublin_core"].(map[string]interface{})["subject"].(string)
	abbreviation = manifest["dublin_core"].(map[string]interface{})["identifier"].(string)
	title = manifest["dublin_core"].(map[string]interface{})["title"].(string)
	language = manifest["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string)
	languageTitle = manifest["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["title"].(string)
	format = manifest["dublin_core"].(map[string]interface{})["format"].(string)
	languageDirection = dcs.GetLanguageDirection(language)
	languageIsGL = dcs.LanguageIsGL(language)
	var bookPath string
	for _, prod := range manifest["projects"].([]interface{}) {
		if prodMap, ok := prod.(map[string]interface{}); ok {
			ingredient := convert.ToIngredient(prodMap)
			book := ingredient.Identifier
			ingredient.Sort = dcs.GetBookSort(book)
			ingredient.Categories = dcs.GetBookCategories(book)
			bookPath = ingredient.Path
			if subject == "Aligned Bible" && strings.HasSuffix(ingredient.Path, ".usfm") {
				count, _ := GetBookAlignmentCount(ingredient.Path, commit)
				ingredient.AlignmentCount = &count
			}
			ingredients = append(ingredients, ingredient)
		}
	}
	if subject == "Bible" || subject == "Aligned Bible" || subject == "Greek New Testament" || subject == "Hebrew Old Testament" {
		contentFormat = "usfm"
		flavorType = "scripture"
		flavor = "textTranslation"
	} else if strings.HasPrefix(subject, "TSV ") {
		if strings.HasPrefix(fmt.Sprintf("./%s_", abbreviation), bookPath) {
			contentFormat = "tsv7"
		} else {
			contentFormat = "tsv9"
		}
		flavorType = "parascriptural"
		flavor = "x-" + strings.Replace(strings.TrimPrefix(subject, "TSV "), " ", "", -1)
	} else if subject == "Open Bible Stories" {
		contentFormat = "markdown"
		flavorType = "gloss"
		flavor = "textStories"
	} else {
		if strings.Contains(format, "/") {
			contentFormat = strings.Split(format, "/")[1]
		} else if repo.PrimaryLanguage != nil {
			contentFormat = strings.ToLower(repo.PrimaryLanguage.Language)
		} else {
			contentFormat = "markdown"
		}
		flavor = "gloss"
		flavor = "x-" + strings.Replace(subject, " ", "", -1)
	}
	var ok bool
	checkingLevel, ok = manifest["checking"].(map[string]interface{})["checking_level"].(int)
	if !ok {
		cL, ok := manifest["checking"].(map[string]interface{})["checking_level"].(string)
		if !ok {
			checkingLevel = 1
		} else {
			var err error
			checkingLevel, err = strconv.Atoi(cL)
			if err != nil {
				checkingLevel = 1
			}
		}
	}

	dm.RepoID = repo.ID
	dm.MetadataType = metadataType
	dm.MetadataVersion = metadataVersion
	dm.Subject = subject
	dm.FlavorType = flavorType
	dm.Flavor = flavor
	dm.Title = title
	dm.Abbreviation = abbreviation
	dm.Language = language
	dm.LanguageTitle = languageTitle
	dm.LanguageDirection = languageDirection
	dm.LanguageIsGL = languageIsGL
	dm.ContentFormat = contentFormat
	dm.CheckingLevel = checkingLevel
	dm.Ingredients = ingredients
	dm.Metadata = manifest

	return nil
}

// GetDoor43MetadataFromSBMetadata creates a Door43Metadata object from the SBMetadata100 object
func GetDoor43MetadataFromSBMetadata(dm *repo_model.Door43Metadata, sbMetadata *dcs.SBMetadata100, repo *repo_model.Repository, commit *git.Commit) error {
	var metadataType string
	var metadataVersion string
	var subject string
	var flavorType string
	var flavor string
	var abbreviation string
	var title string
	var language string
	var languageTitle string
	var languageDirection string
	var languageIsGL bool
	var contentFormat string
	checkingLevel := 1
	var ingredients []*structs.Ingredient

	metadataType = "sb"
	metadataVersion = sbMetadata.Meta.Version
	title = sbMetadata.Identification.Name.DetermineLocalizedTextToUse()
	flavorType = sbMetadata.Type.FlavorType.Name
	flavor = sbMetadata.Type.FlavorType.Flavor.Name
	abbreviation = strings.ToLower(sbMetadata.Identification.Abbreviation.DetermineLocalizedTextToUse())

	switch sbMetadata.Type.FlavorType.Name {
	case "scripture":
		if strings.HasPrefix(flavor, "x-") {
			subject = strings.ToTitle(strings.TrimPrefix(flavor, "x-"))
		} else if flavor == "textTranslation" {
			subject = "Bible"
		} else {
			subject = flavor
		}
		var contentFormat string
		if sbMetadata.Ingredients != nil {
			for filePath, ingredient := range sbMetadata.Ingredients {
				if ingredient.Scope != nil && len(*ingredient.Scope) > 0 {
					bookID := ingredient.Scope.GetBookID()
					var ln *dcs.SB100LocalizedName
					if value, ok := sbMetadata.LocalizedNames[bookID]; ok {
						ln = value
					}
					if ln == nil {
						continue
					}
					filePath = "./" + filePath
					count := 0
					if strings.HasSuffix(filePath, ".usfm") {
						count, _ = GetBookAlignmentCount(filePath, commit)
						if count > 0 && subject == "Bible" {
							subject = "Aligned Bible"
						}
						contentFormat = "usfm"
					} else if contentFormat == "" {
						contentFormat = strings.TrimPrefix(filepath.Ext(filePath), ".")
					}
					ingredients = append(ingredients, &structs.Ingredient{
						Categories:     dcs.GetBookCategories(bookID),
						Identifier:     bookID,
						Title:          ln.Short.DetermineLocalizedTextToUse(),
						Path:           filePath,
						Sort:           dcs.GetBookSort(bookID),
						AlignmentCount: &count,
					})
				}
			}
		} else if sbMetadata.LocalizedNames != nil {
			contentFormat = "usfm"
			for bookID, localizedName := range sbMetadata.LocalizedNames {
				filePath := "./ingredients/" + bookID + ".usfm"
				count, _ := GetBookAlignmentCount(filePath, commit)
				if count > 0 {
					subject = "Aligned Bible"
				}
				ingredients = append(ingredients, &structs.Ingredient{
					Categories:     dcs.GetBookCategories(bookID),
					Identifier:     bookID,
					Title:          localizedName.Short.DetermineLocalizedTextToUse(),
					Path:           filePath,
					Sort:           dcs.GetBookSort(bookID),
					AlignmentCount: &count,
				})
			}
		}
	case "gloss":
		switch sbMetadata.Type.FlavorType.Flavor.Name {
		case "textStories":
			subject = "Open Bible Stories"
			contentFormat = "markdown"
			ingredients = append(ingredients, &structs.Ingredient{
				Identifier: "obs",
				Title:      title,
				Path:       "./ingredients",
			})
		}
	}

	if len(sbMetadata.Languages) > 0 {
		language = sbMetadata.Languages[0].Tag
		languageTitle = dcs.GetLanguageTitle(language)
		if languageTitle == "" {
			languageTitle = sbMetadata.Languages[0].Name.DetermineLocalizedTextToUse()
		}
		languageDirection = dcs.GetLanguageDirection(language)
		languageIsGL = dcs.LanguageIsGL(language)
	}

	dm.RepoID = repo.ID
	dm.MetadataType = metadataType
	dm.MetadataVersion = metadataVersion
	dm.Subject = subject
	dm.FlavorType = flavorType
	dm.Flavor = flavor
	dm.Title = title
	dm.Abbreviation = abbreviation
	dm.Language = language
	dm.LanguageTitle = languageTitle
	dm.LanguageDirection = languageDirection
	dm.LanguageIsGL = languageIsGL
	dm.ContentFormat = contentFormat
	dm.CheckingLevel = checkingLevel
	dm.Ingredients = ingredients
	dm.Metadata = sbMetadata.Metadata

	return nil
}

func GetRCDoor43Metadata(dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	var manifest map[string]interface{}

	blob, err := commit.GetBlobByPath("manifest.yaml")
	if err != nil {
		return err
	}
	if blob == nil {
		return nil
	}
	manifest, err = dcs.ReadYAMLFromBlob(blob)
	if err != nil {
		return err
	}
	validationResult, err := dcs.ValidateMapByRC02Schema(manifest)
	if err != nil {
		return err
	}
	if validationResult != nil {
		log.Info("%s: manifest.yaml is not valid. see errors:", repo.FullName())
		log.Info(dcs.ConvertValidationErrorToString(validationResult))
		return validationResult
	}
	log.Info("%s: manifest.yaml is valid.", repo.FullName())
	return GetDoor43MetadataFromRCManifest(dm, manifest, repo, commit)
}

func GetTcOrTsDoor43Metadata(dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	blob, err := commit.GetBlobByPath("manifest.json")
	if err != nil || blob == nil {
		return err
	}

	log.Info("%s/%s: manifest.json exists so might be a tC or tS repo", repo.FullName(), commit.ID)
	var bookPath string
	var contentFormat string
	var count int
	var versification string

	t, err := dcs.GetTcTsManifestFromBlob(blob)
	if err != nil || t == nil {
		return err
	}
	if t.MetadataType == "ts" {
		bookPath = "."
		contentFormat = "text"
		if t.Project.ID != "obs" {
			versification = "ufw"
		}
	} else {
		bookPath = "./" + repo.Name + ".usfm"
		count, _ = GetBookAlignmentCount(bookPath, commit)
		contentFormat = "usfm"
		versification = "ufw"
	}

	if !dcs.IsValidBook(t.Project.ID) {
		return fmt.Errorf("%s does not have a valid book in its manifest.json", repo.FullName())
	}

	// Get the manifest again in map[string]interface{} format for the DM object
	manifest, err := dcs.ReadJSONFromBlob(blob)
	if err != nil {
		return err
	}

	dm.RepoID = repo.ID
	dm.Repo = repo
	dm.MetadataType = t.MetadataType
	dm.MetadataVersion = t.MetadataVersion
	dm.Subject = t.Subject
	dm.Title = t.Title
	dm.Abbreviation = strings.ToLower(t.Resource.ID)
	dm.Language = t.TargetLanguage.ID
	dm.LanguageTitle = t.TargetLanguage.Name
	dm.LanguageDirection = t.TargetLanguage.Direction
	dm.LanguageIsGL = dcs.LanguageIsGL(t.TargetLanguage.ID)
	dm.ContentFormat = contentFormat
	dm.CheckingLevel = 1
	dm.Ingredients = []*structs.Ingredient{{
		Categories:     dcs.GetBookCategories(t.Project.ID),
		Identifier:     t.Project.ID,
		Title:          t.Project.Name,
		Path:           bookPath,
		Sort:           dcs.GetBookSort(t.Project.ID),
		Versification:  versification,
		AlignmentCount: &count,
	}}
	dm.Metadata = manifest

	return nil
}

func GetSBDoor43Metadata(dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	var metadata map[string]interface{}

	blob, err := commit.GetBlobByPath("metadata.json")
	if err != nil {
		return err
	}
	if blob == nil {
		return nil
	}
	sbMetadata, err := dcs.GetSBDataFromBlob(blob)
	if err != nil {
		log.Error("ERROR: %v", err)
		return err
	}
	metadata, err = dcs.ReadJSONFromBlob(blob)
	if err != nil {
		return err
	}
	validationResult, err := dcs.ValidateMapBySB100Schema(metadata)
	if err != nil {
		return err
	}
	if validationResult != nil {
		log.Info("%s: metadata.json is not valid. see errors:", repo.FullName())
		log.Info(dcs.ConvertValidationErrorToString(validationResult))
		return validationResult
	}
	log.Info("%s: metadata.json is valid.", repo.FullName())

	return GetDoor43MetadataFromSBMetadata(dm, sbMetadata, repo, commit)
}

func processDoor43MetadataForRepoRef(ctx context.Context, repo *repo_model.Repository, ref string) (err error) {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}
	if ref == "" {
		return fmt.Errorf("no ref profided")
	}

	err = repo.LoadLatestDMs(ctx)
	if err != nil {
		return err
	}

	var dm *repo_model.Door43Metadata
	dm, err = repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref)
	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		return err
	}
	if dm == nil {
		dm = &repo_model.Door43Metadata{
			RepoID: repo.ID,
			Ref:    ref,
		}
	}
	dm.Repo = repo

	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		log.Error("OpenRepository Error: %v\n", err)
		return err
	}
	defer gitRepo.Close()

	var commit *git.Commit
	var commitID string
	var stage door43metadata.Stage
	var releaseDateUnix timeutil.TimeStamp
	var releaseID int64

	release, err := repo_model.GetRelease(ctx, repo.ID, ref)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		return err
	}
	if release != nil {
		// We don't support releases that are just tags or are drafts
		if release.IsTag || release.IsDraft {
			return fmt.Errorf("ref for repo %s [%d] must be a branch or a (pre-)release: %s", repo.FullName(), repo.ID, ref)
		}
		if !release.IsCatalogVersion() {
			return fmt.Errorf("release tag for repo %s [%d] must start with v and a digit or be a year: %s", repo.FullName(), repo.ID, release.TagName)
		}
		dm.RefType = "tag"
		dm.Release = release
		if release.IsPrerelease {
			stage = door43metadata.StagePreProd
		} else {
			stage = door43metadata.StageProd
		}
		commit, err = gitRepo.GetTagCommit(ref)
		if err != nil {
			log.Error("GetTagCommit [%s/%s]: %v\n", repo.FullName(), ref, err)
			return err
		}
		commitID = commit.ID.String()
		releaseDateUnix = release.CreatedUnix
		releaseID = release.ID
	} else {
		if branch, err := gitRepo.GetBranch(ref); err != nil && !git.IsErrBranchNotExist(err) {
			return err
		} else if branch == nil {
			return fmt.Errorf("ref for repo %s [%d] does not exist: %s", repo.FullName(), repo.ID, ref)
		}
		if ref == repo.DefaultBranch {
			stage = door43metadata.StageLatest
		} else {
			stage = door43metadata.StageBranch
		}
		dm.IsLatestForStage = true
		dm.RefType = "branch"
		commit, err = gitRepo.GetBranchCommit(ref)
		if err != nil {
			log.Error("GetBranchCommit: %v\n", err)
			return err
		}
		commitID = commit.ID.String()
		releaseDateUnix = timeutil.TimeStamp(commit.Author.When.Unix())
	}

	// Check for SB (Scripture Burrito)
	err = GetSBDoor43Metadata(dm, repo, commit)
	if err != nil {
		if !git.IsErrNotExist(err) {
			log.Info("processDoor43MetadataForRef: ERROR! Unable to populate DM for %s/%s/metadata.json for SB: %v\n", repo.FullName(), ref, err)
			return err
		}
	}

	// Check for TC or TS
	if err != nil {
		err = GetTcOrTsDoor43Metadata(dm, repo, commit)
		if err != nil {
			if !git.IsErrNotExist(err) {
				log.Info("processDoor43MetadataForRef: ERROR! Unable to populate DM for %s/%s/manifest.json for TS or TC: %v\n", repo.FullName(), ref, err)
				return err
			}
		}
	}

	// Check for RC
	if err != nil {
		err = GetRCDoor43Metadata(dm, repo, commit)
		if err != nil {
			if !git.IsErrNotExist(err) {
				log.Info("processDoor43MetadataForRef: ERROR! Unable to populate DM for %s/%s/manifest.yaml for RC: %v\n", repo.FullName(), ref, err)
				return err
			}
			log.Info("processDoor43MetadataForRef: %s/%s is not a SB, TC, TS nor RC repo. Not adding to door43_metadata\n", repo.FullName(), ref)
			return nil // nothing to process, not a SB, TC, TS nor RC repo
		}
	}

	dm.CommitSHA = commitID
	dm.ReleaseID = releaseID
	dm.Release = release
	dm.ReleaseDateUnix = releaseDateUnix
	dm.Stage = stage

	if dm.ID > 0 {
		err = repo_model.UpdateDoor43Metadata(ctx, dm)
		if err != nil {
			return err
		}
	} else {
		err = repo_model.InsertDoor43Metadata(ctx, dm)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateDoor43Metadata generates door43_metadata table entries for valid repos/releases that don't have them
func UpdateDoor43Metadata(ctx context.Context) error {
	log.Trace("Doing: UpdateDoor43Metadata")

	repos, err := repo_model.GetReposForMetadata(ctx)
	if err != nil {
		log.Error("GetReposForMetadata: %v", err)
	}

	for _, repo := range repos {
		if err := ProcessDoor43MetadataForRepo(ctx, repo, ""); err != nil {
			log.Info("Failed to process metadata for repo (%v): %v", repo, err)
			if err = system.CreateRepositoryNotice("Failed to process metadata for repository (%s): %v", repo.FullName(), err); err != nil {
				log.Error("ProcessDoor43MetadataForRepo: %v", err)
			}
		}
	}
	log.Trace("Finished: UpdateDoor43Metadata")
	return nil
}

func DeleteDoor43MetadataByRepoRef(ctx context.Context, repo *repo_model.Repository, ref string) error {
	err := repo_model.DeleteDoor43MetadataByRepoRef(ctx, repo, ref)
	if err != nil {
		return err
	}

	return processDoor43MetadataForRepoLatestDMs(ctx, repo)
}

func UnpackJSONAttachments(ctx context.Context, release *repo_model.Release) {
	if release == nil || len(release.Attachments) == 0 {
		return
	}
	jsonFileNameSuffix := regexp.MustCompile(`(file|link)s*\.json$`)
	for _, attachment := range release.Attachments {
		if jsonFileNameSuffix.MatchString(attachment.Name) {
			remoteAttachments, err := GetAttachmentsFromJSON(attachment)
			if err != nil {
				log.Error("GetAttachmentsFromJSON Error: %v", err)
				continue
			}
			for _, remoteAttachment := range remoteAttachments {
				remoteAttachment.ReleaseID = attachment.ReleaseID
				remoteAttachment.RepoID = attachment.RepoID
				remoteAttachment.UploaderID = attachment.UploaderID
				foundExisting := false
				for _, a := range release.Attachments {
					if a.Name == remoteAttachment.Name {
						if remoteAttachment.Size > 0 {
							a.Size = remoteAttachment.Size
						}
						if remoteAttachment.BrowserDownloadURL != "" {
							a.BrowserDownloadURL = remoteAttachment.BrowserDownloadURL
						}
						a.BrowserDownloadURL = remoteAttachment.BrowserDownloadURL
						if err := repo_model.UpdateAttachment(ctx, a); err != nil {
							log.Error("UpdateAttachment [%d]: %v", a.ID, err)
							continue
						}
						foundExisting = true
						break
					}
				}
				if foundExisting {
					continue
				}
				// No existing attachment was found with the same name, so we insert a new one
				remoteAttachment.UUID = uuid.New().String()
				if _, err = db.GetEngine(db.DefaultContext).Insert(remoteAttachment); err != nil {
					log.Error("insert attachment [%d]: %v", remoteAttachment.ID, err)
					continue
				}
			}
			if err := repo_model.DeleteAttachment(ctx, attachment, true); err != nil {
				log.Error("delete attachment [%d]: %v", attachment.ID, err)
				continue
			}
			continue
		}
	}
}

// GetAttachmentsFromJSON gets the attachments from uploaded
func GetAttachmentsFromJSON(attachment *repo_model.Attachment) ([]*repo_model.Attachment, error) {
	var url string
	if setting.Attachment.Storage.MinioConfig.ServeDirect {
		// If we have a signed url (S3, object storage), redirect to this directly.
		urlObj, err := storage.Attachments.URL(attachment.RelativePath(), attachment.Name)

		if urlObj != nil && err == nil {
			url = urlObj.String()
		}
	} else {
		url = attachment.DownloadURL()
	}
	client := http.Client{
		Timeout: time.Second * 2, // Timeout after 2 seconds
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest Error: %v", err)
	}
	req.Header.Set("User-Agent", "dcs")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client.Do Error: %v", err)
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("client.Do Error: `%s` returned StatusCode [%d]", attachment.DownloadURL(), res.StatusCode)
	}
	if res.Body != nil {
		defer res.Body.Close()
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("io.ReadAll Error: %v", err)
	}
	attachments := []*repo_model.Attachment{}
	if err1 := json.Unmarshal(body, &attachments); err1 != nil {
		// We couldn't unmarshal an array of attachments, so lets see if it is just a single attachment
		attachment := &repo_model.Attachment{}
		if err2 := json.Unmarshal(body, attachment); err2 != nil {
			return nil, fmt.Errorf("json.Unmarshal Error: %v", err1)
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// LoadMetadataSchemas loads the Metadata Schemas from the web and local file if not available online
func LoadMetadataSchemas(ctx context.Context) error {
	log.Trace("Doing: LoadMetadataSchemas")
	if _, err := dcs.GetSB100Schema(true); err != nil {
		log.Error("Error loading SB 100 Schema: %v", err)
	}
	if _, err := dcs.GetRC02Schema(true); err != nil {
		log.Error("Error loading RC 0.2 Schema: %v", err)
	}
	log.Trace("Finished: LoadMetadataSchemas")
	return nil
}
