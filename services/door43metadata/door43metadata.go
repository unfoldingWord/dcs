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

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
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
	"code.gitea.io/gitea/services/door43healthcheck"

	"github.com/google/uuid"
	text_cases "golang.org/x/text/cases"
	text_language "golang.org/x/text/language"
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
		if _, err := processDoor43MetadataForRepoRef(ctx, repo, ref); err != nil {
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
		if dm != nil && dm.ValidationError != nil {
			dm = nil
		}
	} else {
		dm, err = repo_model.GetMostRecentDoor43MetadataByStage(ctx, repo.ID, stage)
	}

	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		return nil, err
	}

	if dm != nil && dm.ValidationError == nil && (earliestDate == nil || dm.ReleaseDateUnix > *earliestDate) {
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
		repo.RepoDM, _ = repo_model.GetMostRecentDoor43MetadataByStage(ctx, repo.ID, door43metadata.StageOther)
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

// processDoor43MetadataForUser determines the given user's languages, subjects, and metadata_types and puts them in those user fields to save to DB
func processDoor43MetadataForUser(ctx context.Context, user *user_model.User) error {
	if user == nil {
		return fmt.Errorf("no user provided")
	}

	user.RepoLanguages = models.GetRepoLanguages(ctx, user)
	user.RepoSubjects = models.GetRepoSubjects(ctx, user)
	user.RepoMetadataTypes = models.GetRepoMetadataTypes(ctx, user)

	return user_model.UpdateUserCols(ctx, user, "repo_languages", "repo_subjects", "repo_metadata_types")
}

// ProcessDoor43MetadataForRepo handles the metadata for a given repo for all its releases
func ProcessDoor43MetadataForRepo(ctx context.Context, repo *repo_model.Repository, ref string) error {
	if ctx == nil || repo == nil {
		return fmt.Errorf("no repository provided")
	}

	if repo.IsArchived || repo.IsPrivate || repo.IsMirror || repo.IsEmpty {
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
			if !git.IsErrNotExist(err) {
				log.Error("processDoor43MetadataForRepoRefs %s Error: %v", repo.FullName(), err)
			}
		}
	} else if _, err := processDoor43MetadataForRepoRef(ctx, repo, ref); err != nil {
		// log error but keep on going
		if !git.IsErrNotExist(err) {
			log.Error("processDoor43MetadataForRepoRef %s Error: %v", repo.FullName(), err)
		}
	}

	err := processDoor43MetadataForRepoLatestDMs(ctx, repo)
	if err != nil {
		return err
	}
	err = repo.LoadOwner(ctx)
	if err != nil {
		return err
	}
	err = processDoor43MetadataForUser(ctx, repo.Owner)
	if err != nil {
		return err
	}

	repo.LoadLatestDMs(ctx)
	if repo.DefaultBranchDM != nil {
		door43healthcheck.RunHealthcheck(ctx, repo.DefaultBranchDM)
	}

	return nil
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

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc), charset.ConvertOpts{})
	buf, err = io.ReadAll(rd)
	if err != nil {
		log.Error("io.ReadAll Error: %v", err)
		return 0, err
	}
	matches := regexp.MustCompile(`\\zaln-s`).FindAllStringIndex(string(buf), -1)
	return len(matches), nil
}

// GetBooks get the books of the manifest
func GetBooks(manifest map[string]any) []string {
	var books []string
	if len((manifest)["projects"].([]any)) > 0 {
		for _, prod := range (manifest)["projects"].([]any) {
			books = append(books, prod.(map[string]any)["identifier"].(string))
		}
	}
	return books
}

func GetDoor43MetadataFromRCManifest(ctx context.Context, dm *repo_model.Door43Metadata, manifest map[string]any, repo *repo_model.Repository, commit *git.Commit) error {
	var metadataType string
	var metadataVersion string
	var subject string
	var flavorType string
	var flavor string
	var abbreviation string
	var title string
	var publisher string
	var language string
	var languageTitle string
	var languageDirection string
	var languageIsGL bool
	var format string
	var contentFormat string
	var checkingLevel int
	var ingredients []*structs.Ingredient
	var relations []*structs.Relation

	repo.LoadOwner(ctx)
	re := regexp.MustCompile("^([^0-9]+)(.*)$")
	matches := re.FindStringSubmatch(manifest["dublin_core"].(map[string]any)["conformsto"].(string))
	if len(matches) == 3 {
		metadataType = matches[1]
		metadataVersion = matches[2]
	} else {
		// should never get here since schema validated
		metadataType = "rc"
		metadataVersion = "0.2"
	}
	subject = manifest["dublin_core"].(map[string]any)["subject"].(string)
	abbreviation = manifest["dublin_core"].(map[string]any)["identifier"].(string)
	title = manifest["dublin_core"].(map[string]any)["title"].(string)
	publisher = manifest["dublin_core"].(map[string]any)["publisher"].(string)
	if publisher == "" {
		publisher = repo.Owner.FullName
		if publisher == "" {
			publisher = repo.OwnerName
		}
	}
	language = manifest["dublin_core"].(map[string]any)["language"].(map[string]any)["identifier"].(string)
	languageTitle = manifest["dublin_core"].(map[string]any)["language"].(map[string]any)["title"].(string)
	format = manifest["dublin_core"].(map[string]any)["format"].(string)
	languageDirection = dcs.GetLanguageDirection(language)
	languageIsGL = dcs.LanguageIsGL(language)
	var bookPath string
	for _, prod := range manifest["projects"].([]any) {
		if prodMap, ok := prod.(map[string]any); ok {
			ingredient := convert.ToIngredient(prodMap)
			book := ingredient.Identifier
			ingredient.Sort = dcs.GetBookSort(book)
			ingredient.Categories = dcs.GetBookCategories(book)
			bookPath = ingredient.Path
			if subject == "Aligned Bible" && strings.HasSuffix(ingredient.Path, ".usfm") {
				count, _ := GetBookAlignmentCount(ingredient.Path, commit)
				ingredient.AlignmentCount = &count
			}
			if entry, err := commit.GetTreeEntryByPath(ingredient.Path); err == nil {
				ingredient.Exists = true
				ingredient.IsDir = entry.IsDir()
				ingredient.Size = entry.Size()
			}
			ingredients = append(ingredients, ingredient)
		}
	}
	for _, relation := range manifest["dublin_core"].(map[string]any)["relation"].([]any) {
		parts := strings.Split(relation.(string), "/")
		lang := parts[0]
		if len(parts) > 1 {
			identifierParts := strings.Split(parts[1], "?v=")
			identifier := identifierParts[0]
			var version string
			if len(identifierParts) > 1 {
				version = identifierParts[1]
			}
			relations = append(relations, &structs.Relation{
				FullRelation: relation.(string),
				Language:     lang,
				Identifier:   identifier,
				Version:      version,
			})
		}
	}
	if subject == "Bible" || subject == "Aligned Bible" || subject == "Greek New Testament" || subject == "Hebrew Old Testament" {
		contentFormat = "usfm"
		flavorType = "scripture"
		flavor = "textTranslation"
	} else if strings.HasPrefix(subject, "TSV ") {
		if strings.HasPrefix(bookPath, fmt.Sprintf("./%s_", abbreviation)) {
			contentFormat = "tsv7"
		} else {
			contentFormat = "tsv9"
		}
		flavorType = "parascriptural"

		switch subject {
		case "TSV Translation Notes":
			flavor = "x-bcvnotes"
		case "TSV Translation Questions":
			flavor = "x-bcvquestions"
		case "TSV Translation Words Links":
			flavor = "x-bcvbcvarticles"
		default:
			flavor = "x-" + strings.ToLower(strings.Fields(subject)[len(strings.Fields(subject))-1])
		}
	} else {
		if strings.Contains(format, "/") {
			contentFormat = strings.Split(format, "/")[1]
		} else if repo.PrimaryLanguage != nil {
			contentFormat = strings.ToLower(repo.PrimaryLanguage.Language)
		} else {
			contentFormat = "markdown"
		}

		switch subject {
		case "Open Bible Stories":
			flavorType = "gloss"
			flavor = "textStories"
		case "Translation Academy", "Translation Words":
			flavorType = "peripheral"
			flavor = "x-peripheralArticles"
		default:
			flavorType = "peripheral"
			flavor = "x-" + strings.ReplaceAll(subject, " ", "")
		}
	}
	var ok bool
	checkingLevel, ok = manifest["checking"].(map[string]any)["checking_level"].(int)
	if !ok {
		cL, ok := manifest["checking"].(map[string]any)["checking_level"].(string)
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
	dm.Publisher = publisher
	dm.Abbreviation = abbreviation
	dm.Language = language
	dm.LanguageTitle = languageTitle
	dm.LanguageDirection = languageDirection
	dm.LanguageIsGL = languageIsGL
	dm.ContentFormat = contentFormat
	dm.CheckingLevel = checkingLevel
	dm.Ingredients = ingredients
	dm.Relations = relations

	return nil
}

// GetDoor43MetadataFromSBMetadata creates a Door43Metadata object from the SBMetadata100 object
func GetDoor43MetadataFromSBMetadata(ctx context.Context, dm *repo_model.Door43Metadata, sbMetadata *dcs.SBMetadata100, repo *repo_model.Repository, commit *git.Commit) error {
	if dm == nil {
		return fmt.Errorf("no Door43Metadata destination provided")
	}
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}
	if sbMetadata == nil {
		return fmt.Errorf("no SB metadata provided")
	}

	var metadataType string
	var publisher string
	var metadataVersion string
	var flavorType string
	var flavor string
	var abbreviation string
	var title string
	var language string
	var languageTitle string
	var languageDirection string
	var languageIsGL bool
	var contentFormat string
	var ingredients []*structs.Ingredient
	checkingLevel := 1
	subject := "Unknown"

	repo.LoadOwner(ctx)
	publisher = repo.Owner.FullName
	if publisher == "" {
		publisher = repo.Owner.Name
	}

	metadataType = "sb"
	if sbMetadata.Meta != nil {
		metadataVersion = sbMetadata.Meta.Version
	}
	if sbMetadata.Identification != nil {
		title = sbMetadata.Identification.Name.DetermineLocalizedTextToUse()
		abbreviation = strings.ToLower(sbMetadata.Identification.Abbreviation.DetermineLocalizedTextToUse())
	}
	if sbMetadata.Type != nil {
		flavorType = sbMetadata.Type.FlavorType.Name
		flavor = sbMetadata.Type.FlavorType.Flavor.Name
	}

	for _, lang := range sbMetadata.Languages {
		if lang == nil {
			continue
		}
		language = lang.Tag
		languageTitle = dcs.GetLanguageTitle(language)
		if languageTitle == "" {
			languageTitle = lang.Name.DetermineLocalizedTextToUse()
		}
		break
	}
	languageDirection = dcs.GetLanguageDirection(language)
	languageIsGL = dcs.LanguageIsGL(language)

	switch flavorType {
	case "scripture":
		if after, ok := strings.CutPrefix(flavor, "x-"); ok {
			subject = text_cases.Title(text_language.English).String(after)
		} else if flavor == "textTranslation" {
			subject = "Bible"
		}
		for filePath, ingredient := range sbMetadata.Ingredients {
			bookID, lowerBookID, ok := getSBIngredientBookID(ingredient)
			if !ok {
				continue
			}
			normalizedPath := normalizeSBIngredientPath(filePath)
			count := 0
			if strings.HasSuffix(strings.ToLower(normalizedPath), ".usfm") {
				count, _ = getBookAlignmentCountSafe(normalizedPath, commit)
				if count > 0 && subject == "Bible" {
					subject = "Aligned Bible"
				}
				contentFormat = "usfm"
			} else if contentFormat == "" {
				contentFormat = strings.TrimPrefix(strings.ToLower(filepath.Ext(normalizedPath)), ".")
			}
			ingredients = append(ingredients, &structs.Ingredient{
				Categories:     dcs.GetBookCategories(lowerBookID),
				Identifier:     lowerBookID,
				Title:          getSBLocalizedBookTitle(sbMetadata.LocalizedNames, bookID, lowerBookID),
				Path:           normalizedPath,
				Sort:           dcs.GetBookSort(lowerBookID),
				Versification:  "ufw",
				AlignmentCount: &count,
			})
		}
	case "gloss":
		switch flavor {
		case "textStories":
			subject = "Open Bible Stories"
			contentFormat = "markdown"
			ingredients = append(ingredients, &structs.Ingredient{
				Identifier: "obs",
				Title:      title,
				Path:       "./ingredients",
			})
		}
	case "parascriptural":
		if strings.HasPrefix(flavor, "x-bcv") {
			contentFormat = "tsv7"
			switch strings.ToLower(flavor) {
			case "x-bcvnotes":
				subject = "TSV Translation Notes"
			case "x-bcvquestions":
				subject = "TSV Translation Questions"
			case "x-bcvbcvarticles":
				subject = "TSV Translation Words Links"
			}

			for path, ingredient := range sbMetadata.Ingredients {
				bookID, lowerBookID, ok := getSBIngredientBookID(ingredient)
				if !ok {
					continue
				}
				ingredients = append(ingredients, &structs.Ingredient{
					Identifier:    lowerBookID,
					Title:         getSBLocalizedBookTitle(sbMetadata.LocalizedNames, bookID, lowerBookID),
					Path:          normalizeSBIngredientPath(path),
					Sort:          dcs.GetBookSort(lowerBookID),
					Versification: "ufw",
				})
			}
		}
	case "peripheral":
		switch strings.ToLower(flavor) {
		case "x-peripheralarticles":
			contentFormat = "markdown"
			switch strings.ToLower(abbreviation) {
			case "ta":
				subject = "Translation Academy"
				contentFormat = "markdown"
				ingredients = append(ingredients, &structs.Ingredient{
					Path:       "./ingredients/intro",
					Identifier: "intro",
					Title:      "Introduction to Translation Academy",
					Sort:       0,
				})
				ingredients = append(ingredients, &structs.Ingredient{
					Path:       "./ingredients/process",
					Identifier: "process",
					Title:      "Process Manual",
					Sort:       1,
				})
				ingredients = append(ingredients, &structs.Ingredient{
					Path:       "./ingredients/translate",
					Identifier: "translate",
					Title:      "Translation Manual",
					Sort:       2,
				})
				ingredients = append(ingredients, &structs.Ingredient{
					Path:       "./ingredients/checking",
					Identifier: "checking",
					Title:      "Checking Manual",
					Sort:       3,
				})
			case "tw":
				subject = "Translation Words"
				ingredients = append(ingredients, &structs.Ingredient{
					Path:       "./ingredients",
					Identifier: "bible",
					Title:      "Translation Words",
					Sort:       0,
				})
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
	dm.Publisher = publisher
	dm.Language = language
	dm.LanguageTitle = languageTitle
	dm.LanguageDirection = languageDirection
	dm.LanguageIsGL = languageIsGL
	dm.ContentFormat = contentFormat
	dm.CheckingLevel = checkingLevel
	dm.Ingredients = ingredients

	return nil
}

func normalizeSBIngredientPath(path string) string {
	path = strings.TrimPrefix(path, "./")
	if path == "" {
		return "./"
	}
	return "./" + path
}

func getSBIngredientBookID(ingredient *dcs.SB100Ingredient) (bookID, lowerBookID string, ok bool) {
	if ingredient == nil || ingredient.Scope == nil || len(*ingredient.Scope) == 0 {
		return "", "", false
	}
	bookID = ingredient.Scope.GetBookID()
	if bookID == "" {
		return "", "", false
	}
	return bookID, strings.ToLower(bookID), true
}

func getSBLocalizedBookTitle(localizedNames map[string]*dcs.SB100LocalizedName, bookID, lowerBookID string) string {
	if ln := localizedNames[bookID]; ln != nil {
		if title := ln.Short.DetermineLocalizedTextToUse(); title != "" {
			return title
		}
	}
	if title := dcs.GetBookName(lowerBookID); title != "" {
		return title
	}
	if bookID != "" {
		return strings.ToUpper(bookID)
	}
	return lowerBookID
}

func getBookAlignmentCountSafe(bookPath string, commit *git.Commit) (int, error) {
	if commit == nil {
		return 0, nil
	}
	return GetBookAlignmentCount(bookPath, commit)
}

func GetRCDoor43Metadata(ctx context.Context, dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	var manifest map[string]any

	blob, err := commit.GetBlobByPath("manifest.yaml")
	if err != nil {
		return err
	}
	if blob == nil {
		return nil
	}
	manifest, err = dcs.ReadYAMLFromBlob(blob)
	if err != nil {
		log.Error("ReadYAMLFromBlob: %v", err)
		return err
	}
	dm.Metadata = manifest

	dm.ValidationError, err = dcs.ValidateMapByRC02Schema(manifest)
	if err != nil {
		return err
	}
	if dm.ValidationError != nil {
		dm.IsLatestForStage = false
		dm.Stage = door43metadata.StageOther
		log.Debug("%s: manifest.yaml is not valid. see errors:", repo.FullName())
		log.Debug(dcs.ConvertValidationErrorToString(dm.ValidationError))
		return nil
	}
	log.Debug("%s/%s: manifest.yaml is valid", repo.FullName(), dm.Ref)
	return GetDoor43MetadataFromRCManifest(ctx, dm, manifest, repo, commit)
}

func GetTcOrTsDoor43Metadata(dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	blob, err := commit.GetBlobByPath("manifest.json")
	if err != nil || blob == nil {
		return err
	}

	log.Debug("%s/%s (%s): manifest.json exists so might be a tC or tS repo", repo.FullName(), dm.Ref, commit.ID)
	var bookPath string
	var count int
	var versification string

	t, err := dcs.GetTcTsManifestFromBlob(blob)
	if err != nil || t == nil {
		return err
	}
	if t.MetadataType == "ts" {
		bookPath = "."
		if t.Project.ID != "obs" {
			versification = "ufw"
		}
	} else {
		bookPath = "./" + repo.Name + ".usfm"
		count, _ = GetBookAlignmentCount(bookPath, commit)
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
	dm.FlavorType = t.FlavorType
	dm.Flavor = t.Flavor
	dm.Title = t.Title
	dm.Abbreviation = strings.ToLower(t.Resource.ID)
	dm.Language = t.TargetLanguage.ID
	dm.LanguageTitle = t.TargetLanguage.Name
	dm.LanguageDirection = t.TargetLanguage.Direction
	dm.LanguageIsGL = dcs.LanguageIsGL(t.TargetLanguage.ID)
	dm.ContentFormat = t.Format
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

func GetSBDoor43Metadata(ctx context.Context, dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	blob, err := commit.GetBlobByPath("metadata.json")
	if err != nil {
		return err
	}
	if blob == nil {
		return nil
	}
	sbMetadata, err := dcs.GetSBDataFromBlob(blob)
	if err != nil {
		log.Error("GetSBDataFromBlob: %v", err)
		return err
	}
	dm.Metadata = sbMetadata.Metadata

	dm.ValidationError, err = dcs.ValidateMapBySB100Schema(sbMetadata.Metadata)
	if err != nil {
		return err
	}
	if dm.ValidationError != nil {
		dm.IsLatestForStage = false
		dm.Stage = door43metadata.StageOther
		log.Debug("%s/%s: metadata.json is not valid. see errors:", repo.FullName(), dm.Ref)
		log.Debug(dcs.ConvertValidationErrorToString(dm.ValidationError))
		return nil
	}
	log.Debug("%s/%s: metadata.json is valid", repo.FullName(), dm.Ref)

	return GetDoor43MetadataFromSBMetadata(ctx, dm, sbMetadata, repo, commit)
}

func processDoor43MetadataForRepoRef(ctx context.Context, repo *repo_model.Repository, ref string) (dm *repo_model.Door43Metadata, err error) {
	if repo == nil {
		err = fmt.Errorf("no repository provided")
		return
	}
	if ref == "" {
		err = fmt.Errorf("no ref provided")
		return
	}

	if repo.IsArchived || repo.IsEmpty || repo.IsMirror || repo.IsPrivate {
		err = fmt.Errorf("repo must not be empty, an arhcive, a mirror or private")
		return
	}

	err = repo.LoadLatestDMs(ctx)
	if err != nil {
		return
	}

	dm, err = repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref)
	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		return
	}
	if dm == nil {
		dm = &repo_model.Door43Metadata{
			RepoID: repo.ID,
			Ref:    ref,
			Stage:  door43metadata.StageOther,
		}
	}
	if dm.Stage < 1 {
		dm.Stage = door43metadata.StageOther
	}
	dm.Repo = repo

	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		log.Error("OpenRepository Error: %v\n", err)
		return
	}
	defer gitRepo.Close()

	var commit *git.Commit

	dm.Release, err = repo_model.GetRelease(ctx, repo.ID, ref)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		return
	}
	if dm.Release != nil {
		if dm.Release.IsDraft {
			return
		}
		dm.ReleaseID = dm.Release.ID
		dm.RefType = "tag"
		if !dm.Release.IsTag && dm.Release.IsCatalogVersion() {
			if dm.Release.IsPrerelease {
				dm.Stage = door43metadata.StagePreProd
			} else {
				dm.Stage = door43metadata.StageProd
			}
		} else {
			dm.Stage = door43metadata.StageOther
			dm.IsLatestForStage = false
		}
		commit, err = gitRepo.GetTagCommit(ref)
		if err != nil {
			log.Error("GetTagCommit [%s/%s]: %v\n", repo.FullName(), ref, err)
			return
		}
		dm.CommitSHA = commit.ID.String()
		dm.ReleaseDateUnix = dm.Release.CreatedUnix
	} else if !gitRepo.IsBranchExist(ref) {
		err = fmt.Errorf("ref for repo %s [%d] does not exist: %s", repo.FullName(), repo.ID, ref)
		return
	} else {
		dm.Stage = door43metadata.StageOther
		dm.IsLatestForStage = false
		dm.RefType = "branch"
		commit, err = gitRepo.GetBranchCommit(ref)
		if err != nil {
			log.Error("GetBranchCommit: %v\n", err)
			return
		}
		dm.CommitSHA = commit.ID.String()
		dm.ReleaseDateUnix = timeutil.TimeStamp(commit.Author.When.Unix())
	}

	// Check for SB (Scripture Burrito)
	err = GetSBDoor43Metadata(ctx, dm, repo, commit)
	if err != nil && !git.IsErrNotExist(err) {
		log.Debug("processDoor43MetadataForRef: ERROR! Unable to populate DM for %s/%s/metadata.json for SB: %v\n", repo.FullName(), ref, err)
		return
	}

	// Check for TC or TS
	if err != nil {
		err = GetTcOrTsDoor43Metadata(dm, repo, commit)
		if err != nil {
			if !git.IsErrNotExist(err) {
				log.Debug("processDoor43MetadataForRef: ERROR! Unable to populate DM for %s/%s/manifest.json for TS or TC: %v\n", repo.FullName(), ref, err)
				return
			}
		}
	}

	// Check for RC
	if err != nil {
		err = GetRCDoor43Metadata(ctx, dm, repo, commit)
		if err != nil {
			if !git.IsErrNotExist(err) {
				log.Debug("processDoor43MetadataForRef: ERROR! Unable to populate DM for %s/%s/manifest.yaml for RC: %v\n", repo.FullName(), ref, err)
				return
			}
			log.Debug("processDoor43MetadataForRef: %s/%s is not a SB, TC, TS nor RC repo. Not adding to door43_metadata\n", repo.FullName(), ref)
			return // nothing to process, not a SB, TC, TS nor RC repo
		}
	}

	if dm.ID > 0 {
		err = repo_model.UpdateDoor43Metadata(ctx, dm)
		if err != nil {
			return
		}
	} else {
		if dm.ValidationError != nil {
			// We didn't get any properties from the metadata file since it was invalid
			dm.CopyEmptyPropertiesFromRepoDM(ctx)
		}
		err = repo_model.InsertDoor43Metadata(ctx, dm)
		if err != nil {
			return
		}
	}

	return
}

// UpdateUserMetadata updates the user table with their repo langauges, subjects and metadata types
func UpdateUserMetadata(ctx context.Context) error {
	log.Trace("Doing: UpdateUserMetadata")

	var users []*user_model.User
	err := db.GetEngine(ctx).
		Select("`user`.*").
		Join("INNER", "repository", "`repository`.owner_id = `user`.id").
		Join("INNER", "door43_metadata", "`door43_metadata`.repo_id = `repository`.id").
		GroupBy("`user`.id").
		Find(&users)
	if err != nil {
		log.Error("UpdateUserMetadata: %v", err)
	}

	for _, user := range users {
		if err := processDoor43MetadataForUser(ctx, user); err != nil {
			log.Info("Failed to process metadata for user (%v): %v", user, err)
			if err = system.CreateRepositoryNotice("Failed to process metadata for user (%s): %v", user.Name, err); err != nil {
				log.Error("ProcessDoor43MetadataForUser: %v", err)
			}
		}
	}
	log.Trace("Finished: UpdateUserMetadata")
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

func DeleteDoor43MetadataByRepoAndRef(ctx context.Context, repo *repo_model.Repository, ref string) error {
	err := repo_model.DeleteDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref)
	if err != nil {
		log.Error("DeleteDoor43MetadataByRepoIDAndRef %v", err)
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
				if _, err = db.GetEngine(ctx).Insert(remoteAttachment); err != nil {
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
		urlObj, err := storage.Attachments.URL(attachment.RelativePath(), attachment.Name, "", nil)

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
