// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gitea.dev/models"
	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/system"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/dcs"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	"gitea.dev/services/convert"
	"gitea.dev/services/door43healthcheck"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v5"
	text_cases "golang.org/x/text/cases"
	text_language "golang.org/x/text/language"
	"xorm.io/builder"
)

func processDoor43MetadataForRepoRefs(ctx context.Context, repo *repo_model.Repository) error {
	passStart := timeutil.TimeStampNow()
	refsComplete := true

	refs, err := repo_model.GetRepoReleaseTagsForMetadata(ctx, repo.ID)
	if err != nil {
		log.Error("GetRepoReleaseTagsForMetadata Error %s: %v", repo.FullName(), err)
		refsComplete = false
	}

	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		log.Error("git.OpenRepository Error %s: %v", repo.FullName(), err)
		refsComplete = false
	}
	if gitRepo != nil {
		defer gitRepo.Close()
		branchNames, _, err := gitRepo.GetBranchNames(ctx, 0, 0)
		if err != nil {
			log.Error("git.GetBranchNames Error %s: %v", repo.FullName(), err)
			refsComplete = false
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

	// Sweep entries whose ref no longer exists (e.g. a branch deleted while a check was
	// in flight, or a missed delete notification). Only runs when both the tag and
	// branch listings succeeded — a partial ref list must never trigger deletions —
	// and spares rows touched since this pass started (a branch pushed mid-pass).
	if refsComplete {
		if count, err := repo_model.DeleteDoor43MetadatasStaleRefs(ctx, repo.ID, refs, passStart); err != nil {
			log.Error("DeleteDoor43MetadatasStaleRefs %s: %v", repo.FullName(), err)
		} else if count > 0 {
			log.Info("Deleted %d stale door43_metadata entries for refs no longer in %s", count, repo.FullName())
		}
	}
	return nil
}

func handleLatestStageDM(ctx context.Context, repo *repo_model.Repository, stage door43metadata.Stage, earliestDate *timeutil.TimeStamp) (*repo_model.Door43Metadata, error) {
	// The current latest release entry may carry media flags aggregated over all
	// the repo's releases of this stage (repo_model.AggregateMediaFlagsForRepo);
	// remember it so that if it loses latestness below, its flags can be reset to
	// its own release's attachments.
	prevDM := &repo_model.Door43Metadata{}
	hasPrev := false
	if stage != door43metadata.StageLatest {
		var err error
		hasPrev, err = db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repo.ID, "stage": stage, "is_latest_for_stage": true}).
			Get(prevDM)
		if err != nil {
			return nil, err
		}
	}

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

	// Demoted from latest: reset the aggregated media flags to the entry's own
	// release attachments so non-latest entries always carry per-release truth.
	if hasPrev && (dm == nil || dm.ID != prevDM.ID) {
		if err := prevDM.DetermineAttachmentFlags(ctx); err != nil {
			return nil, err
		}
		if err := repo_model.UpdateDoor43MetadataCols(ctx, prevDM, repo_model.Door43MetadataAttachmentFlagCols...); err != nil {
			return nil, err
		}
	}

	return dm, nil
}

func handleRepoDM(ctx context.Context, repo *repo_model.Repository) {
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

	// RepoDM must never be left nil: LoadLatestDMs is memoized via
	// repo.LatestDMsLoaded, so callers like ToRepoDCS that dereference RepoDM
	// would not get another chance to synthesize it (a repo with no valid DMs,
	// e.g. just created without a manifest, would panic otherwise).
	if repo.RepoDM == nil {
		repo.RepoDM = repo_model.SynthesizeRepoDM(repo)
	}
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

	handleRepoDM(ctx, repo)

	// The latest prod/preprod entries carry the media flags of ALL the repo's
	// releases at their stage, keeping stats and has_* filtering correct after
	// any release is created, updated or deleted.
	if err := repo_model.AggregateMediaFlagsForRepo(ctx, repo.ID); err != nil {
		log.Error("AggregateMediaFlagsForRepo [%s]: %v", repo.FullName(), err)
	}

	return nil
}

// processDoor43MetadataForUser determines the given user's languages, subjects, and metadata_types and puts them in those user fields to save to DB
func processDoor43MetadataForUser(ctx context.Context, user *user_model.User) error {
	if user == nil {
		return errors.New("no user provided")
	}

	languages, subjects, metadataTypes, err := models.GetRepoMetadataFacets(ctx, user)
	if err != nil {
		return err
	}
	user.RepoLanguages = languages
	user.RepoSubjects = subjects
	user.RepoMetadataTypes = metadataTypes

	return user_model.UpdateUserCols(ctx, user, "repo_languages", "repo_subjects", "repo_metadata_types")
}

// ProcessDoor43MetadataForRepo handles the metadata for a given repo for all its releases
func ProcessDoor43MetadataForRepo(ctx context.Context, repo *repo_model.Repository, ref string) error {
	return processDoor43MetadataForRepo(ctx, repo, ref, true)
}

// processDoor43MetadataForRepo does the work of ProcessDoor43MetadataForRepo. Bulk
// callers pass updateOwner=false and roll each owner up once themselves: the rollup
// reads all of the owner's entries, so running it per repo repeats the same scan for
// every repo the owner has.
func processDoor43MetadataForRepo(ctx context.Context, repo *repo_model.Repository, ref string, updateOwner bool) error {
	if ctx == nil || repo == nil {
		return errors.New("no repository provided")
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
	} else if err := processDoor43MetadataForRepoRef(ctx, repo, ref); err != nil {
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
	if updateOwner {
		if err = processDoor43MetadataForUser(ctx, repo.Owner); err != nil {
			return err
		}
	}

	_ = repo.LoadLatestDMs(ctx)
	if repo.DefaultBranchDM != nil {
		door43healthcheck.RunHealthcheck(ctx, repo.DefaultBranchDM)
	}

	return nil
}

func GetBookAlignmentCount(ctx context.Context, gitRepo *git.Repository, bookPath string, commit *git.Commit) (int, error) {
	blob, err := commit.GetBlobByPath(ctx, gitRepo, bookPath)
	if err != nil {
		if !git.IsErrNotExist(err) {
			log.Error("GetBlobByPath(%s) Error: %v\n", bookPath, err)
		}
		return 0, err
	}
	dataRc, err := blob.DataAsync(ctx)
	if err != nil {
		log.Error("blob.DataAsync(ctx) Error: %v\n", err)
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
	for _, prod := range dcs.MapSlice(manifest, "projects") {
		if prodMap, ok := prod.(map[string]any); ok {
			books = append(books, dcs.MapStr(prodMap, "identifier"))
		}
	}
	return books
}

func GetDoor43MetadataFromRCManifest(ctx context.Context, gitRepo *git.Repository, dm *repo_model.Door43Metadata, manifest map[string]any, repo *repo_model.Repository, commit *git.Commit) error {
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

	_ = repo.LoadOwner(ctx)
	dublinCore := dcs.MapMap(manifest, "dublin_core")
	re := regexp.MustCompile("^([^0-9]+)(.*)$")
	matches := re.FindStringSubmatch(dcs.MapStr(dublinCore, "conformsto"))
	if len(matches) == 3 {
		metadataType = matches[1]
		metadataVersion = matches[2]
	} else {
		// should never get here since schema validated
		metadataType = "rc"
		metadataVersion = "0.2"
	}
	subject = dcs.MapStr(dublinCore, "subject")
	abbreviation = dcs.MapStr(dublinCore, "identifier")
	title = dcs.MapStr(dublinCore, "title")
	publisher = dcs.MapStr(dublinCore, "publisher")
	if publisher == "" {
		publisher = repo.Owner.FullName
		if publisher == "" {
			publisher = repo.OwnerName
		}
	}
	dcLanguage := dcs.MapMap(dublinCore, "language")
	language = dcs.MapStr(dcLanguage, "identifier")
	languageTitle = dcs.MapStr(dcLanguage, "title")
	format = dcs.MapStr(dublinCore, "format")
	languageDirection = dcs.GetLanguageDirection(language)
	languageIsGL = dcs.LanguageIsGL(language)
	var bookPath string
	for _, prod := range dcs.MapSlice(manifest, "projects") {
		if prodMap, ok := prod.(map[string]any); ok {
			ingredient := convert.ToIngredient(prodMap)
			book := ingredient.Identifier
			ingredient.Sort = dcs.GetBookSort(book)
			ingredient.Categories = dcs.GetBookCategories(book)
			bookPath = ingredient.Path
			if subject == "Aligned Bible" && strings.HasSuffix(ingredient.Path, ".usfm") {
				count, _ := getBookAlignmentCountSafe(ctx, gitRepo, ingredient.Path, commit)
				ingredient.AlignmentCount = &count
			}
			if commit != nil {
				if entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, ingredient.Path); err == nil {
					ingredient.Exists = true
					ingredient.IsDir = entry.IsDir()
					ingredient.Size = entry.GetSize(ctx, gitRepo)
				}
			}
			ingredients = append(ingredients, ingredient)
		}
	}
	for _, relation := range dcs.MapSlice(dublinCore, "relation") {
		relationStr, ok := relation.(string)
		if !ok {
			continue
		}
		parts := strings.Split(relationStr, "/")
		lang := parts[0]
		if len(parts) > 1 {
			identifierParts := strings.Split(parts[1], "?v=")
			identifier := identifierParts[0]
			var version string
			if len(identifierParts) > 1 {
				version = identifierParts[1]
			}
			relations = append(relations, &structs.Relation{
				FullRelation: relationStr,
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
			flavor = "x-bcvarticles"
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
	dm.Language = strings.ToLower(language) // language codes are always stored lowercase
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
func GetDoor43MetadataFromSBMetadata(ctx context.Context, gitRepo *git.Repository, dm *repo_model.Door43Metadata, sbMetadata *dcs.SBMetadata100, repo *repo_model.Repository, commit *git.Commit) error {
	if dm == nil {
		return errors.New("no Door43Metadata destination provided")
	}
	if repo == nil {
		return errors.New("no repository provided")
	}
	if sbMetadata == nil {
		return errors.New("no SB metadata provided")
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
	var subject string

	_ = repo.LoadOwner(ctx)
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
	repoNameSuffix := getSBRepoNameSuffix(repo.Name)
	subject = getSBSubject(flavorType, flavor, abbreviation, repoNameSuffix)

	switch subject {
	case "Bible", "Aligned Bible", "Greek New Testament", "Hebrew Old Testament":
		var hasAlignment bool
		ingredients, contentFormat, hasAlignment = getSBScriptureIngredients(ctx, gitRepo, sbMetadata, commit)
		if subject == "Bible" && hasAlignment {
			subject = "Aligned Bible"
		}
	case "Open Bible Stories":
		contentFormat = "markdown"
		ingredients = []*structs.Ingredient{{
			Identifier: "obs",
			Title:      title,
			Path:       "./ingredients",
			IsDir:      true,
			Exists:     true,
		}}
	case "TSV Translation Notes", "TSV Translation Questions", "TSV Translation Words Links":
		contentFormat = "tsv7"
		ingredients = getSBTSVIngredients(ctx, gitRepo, sbMetadata, commit)
	case "TSV OBS Study Questions", "TSV OBS Translation Questions", "TSV OBS Study Notes", "TSV OBS Translation Notes":
		contentFormat = "tsv7"
		ingredients = getSBOBSTSVIngredients(ctx, gitRepo, sbMetadata, commit)
	case "Translation Academy":
		contentFormat = "markdown"
		ingredients = getSBTranslationAcademyIngredients()
	case "Translation Words":
		contentFormat = "markdown"
		ingredients = []*structs.Ingredient{{
			Path:       "./ingredients",
			Identifier: "bible",
			Title:      "Translation Words",
			Sort:       0,
			IsDir:      true,
			Exists:     true,
		}}
	default:
		// Keep scripture ingredient processing for custom x-* scripture flavors.
		if strings.EqualFold(flavorType, "scripture") {
			ingredients, contentFormat, _ = getSBScriptureIngredients(ctx, gitRepo, sbMetadata, commit)
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
	dm.Language = strings.ToLower(language) // language codes are always stored lowercase
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

func getBookAlignmentCountSafe(ctx context.Context, gitRepo *git.Repository, bookPath string, commit *git.Commit) (int, error) {
	if commit == nil {
		return 0, nil
	}
	return GetBookAlignmentCount(ctx, gitRepo, bookPath, commit)
}

func getSBRepoNameSuffix(repoName string) string {
	if idx := strings.LastIndex(repoName, "_"); idx >= 0 {
		return strings.ToLower(repoName[idx+1:])
	}
	return strings.ToLower(repoName)
}

func getSBSubject(flavorType, flavor, abbreviation, repoNameSuffix string) string {
	flavorLower := strings.ToLower(flavor)
	abbreviationLower := strings.ToLower(abbreviation)

	switch strings.ToLower(flavorType) {
	case "scripture":
		if after, ok := strings.CutPrefix(flavor, "x-"); ok {
			return text_cases.Title(text_language.English).String(after)
		}
		if flavor == "textTranslation" {
			return "Bible" // Leave it as Bible now. If we find alignments later, will be Aligned Bible
		}
	case "gloss":
		if flavor == "textStories" {
			return "Open Bible Stories"
		}
	case "parascriptural":
		switch flavorLower {
		case "x-bcvnotes":
			return "TSV Translation Notes"
		case "x-bcvquestions":
			return "TSV Translation Questions"
		case "x-bcvarticles":
			return "TSV Translation Words Links"
		}
	case "peripheral":
		switch flavorLower {
		case "x-greeklexicon", "x-greeklexicons":
			return "Greek Lexicon"
		case "x-hebrewlexicon", "x-hebrewlexicons":
			return "Hebrew Lexicon"
		case "x-lexicon", "x-lexicons":
			switch abbreviationLower {
			case "hl", "thl", "uhl":
				return "Hebrew Lexicon"
			case "gl", "tgl", "ugl":
				return "Greek Lexicon"
			default:
				switch repoNameSuffix {
				case "ugl":
					return "Greek Lexicon"
				case "uhl":
					return "Hebrew Lexicon"
				}
			}

		case "x-obsstudyquestions":
			return "TSV OBS Study Questions"
		case "x-obstranslationquestions":
			return "TSV OBS Translation Questions"
		case "x-obsquestions":
			switch abbreviationLower {
			case "obstq":
				return "TSV OBS Translation Questions"
			case "obssq":
				return "TSV OBS Study Questions"
			default:
				switch repoNameSuffix {
				case "obs-tq":
					return "TSV OBS Translation Questions"
				case "obs-sq":
					return "TSV OBS Study Questions"
				}
			}
		case "x-obsstudynotes":
			return "TSV OBS Study Notes"
		case "x-obstranslationnotes", "obstn":
			return "TSV OBS Translation Notes"
		case "x-obsnotes":
			switch abbreviationLower {
			case "obstn":
				return "TSV OBS Translation Notes"
			case "obssn":
				return "TSV OBS Study Notes"
			default:
				switch repoNameSuffix {
				case "obs-tn":
					return "TSV OBS Translation Notes"
				case "obs-sn":
					return "TSV OBS Study Notes"
				}
			}
		case "obssn":
			return "TSV OBS Study Notes"
		case "x-obstheologicalformation":
			return "OBS Theological Formation"
		case "x-peripheralarticles", "x-translationacademy", "x-translationwords":
			switch abbreviationLower {
			case "ta":
				return "Translation Academy"
			case "tw":
				return "Translation Words"
			default:
				switch repoNameSuffix {
				case "ta":
					return "Translation Academy"
				case "tw":
					return "Translation Words"
				}
			}
		default:
			switch repoNameSuffix {
			case "sn":
				return "TSV Study Notes"
			case "sq":
				return "TSV Study Questions"
			case "ta":
				return "Translation Academy"
			case "tw":
				return "Translation Words"
			case "tn":
				return "TSV Translation Notes"
			case "tq":
				return "TSV Translation Questions"
			case "twl":
				return "TSV Translation Words Links"
			case "obs":
				return "Open Bible Stories"
			case "obs-tn":
				return "TSV OBS Translation Notes"
			case "obs-tq":
				return "TSV OBS Translation Questions"
			case "obs-sn":
				return "TSV OBS Study Notes"
			case "obs-sq":
				return "TSV OBS Study Questions"
			case "glt", "gst", "ult", "ust":
				return "Aligned Bible"
			case "uhl", "thl", "hl":
				return "Hebrew Lexicon"
			case "ugl", "tgl", "gl":
				return "Greek Lexicon"
			case "ugg", "tgg", "gg":
				return "Greek Grammar"
			case "uhg", "thg", "hg":
				return "Hebrew Grammar"
			case "uag", "tag", "ag":
				return "Aramaic Grammar"
			}
		}
	}

	return "Unknown"
}

func getSBScriptureIngredients(ctx context.Context, gitRepo *git.Repository, sbMetadata *dcs.SBMetadata100, commit *git.Commit) ([]*structs.Ingredient, string, bool) {
	var ingredients []*structs.Ingredient
	contentFormat := ""
	hasAlignment := false

	for filePath, ingredient := range sbMetadata.Ingredients {
		bookID, lowerBookID, ok := getSBIngredientBookID(ingredient)
		if !ok {
			continue
		}
		normalizedPath := normalizeSBIngredientPath(filePath)
		count := 0
		if strings.HasSuffix(strings.ToLower(normalizedPath), ".usfm") {
			count, _ = getBookAlignmentCountSafe(ctx, gitRepo, normalizedPath, commit)
			if count > 0 {
				hasAlignment = true
			}
			contentFormat = "usfm"
		} else if contentFormat == "" {
			contentFormat = strings.TrimPrefix(strings.ToLower(filepath.Ext(normalizedPath)), ".")
		}
		size, isDir := getSBIngredientSize(ctx, gitRepo, normalizedPath, ingredient, commit)
		ingredients = append(ingredients, &structs.Ingredient{
			Categories:     dcs.GetBookCategories(lowerBookID),
			Identifier:     lowerBookID,
			Title:          getSBLocalizedBookTitle(sbMetadata.LocalizedNames, bookID, lowerBookID),
			Path:           normalizedPath,
			Sort:           dcs.GetBookSort(lowerBookID),
			Versification:  "ufw",
			AlignmentCount: &count,
			Size:           size,
			IsDir:          isDir,
			Exists:         true,
		})
	}

	return ingredients, contentFormat, hasAlignment
}

// getSBIngredientSize returns the ingredient file's size and dir flag from the repo, falling
// back to the size metadata.json declares when the path doesn't resolve. A declared size that
// disagrees with the file is reported separately by the sb ingredient healthcheck (META-015).
func getSBIngredientSize(ctx context.Context, gitRepo *git.Repository, path string, ingredient *dcs.SB100Ingredient, commit *git.Commit) (int64, bool) {
	if commit != nil {
		if entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, path); err == nil && entry != nil {
			return entry.GetSize(ctx, gitRepo), entry.IsDir()
		}
	}
	if ingredient == nil {
		return 0, false
	}
	return ingredient.Size, false
}

func getSBTSVIngredients(ctx context.Context, gitRepo *git.Repository, sbMetadata *dcs.SBMetadata100, commit *git.Commit) []*structs.Ingredient {
	ingredients := make([]*structs.Ingredient, 0, len(sbMetadata.Ingredients))
	for path, ingredient := range sbMetadata.Ingredients {
		bookID, lowerBookID, ok := getSBIngredientBookID(ingredient)
		if !ok {
			continue
		}
		normalizedPath := normalizeSBIngredientPath(path)
		size, isDir := getSBIngredientSize(ctx, gitRepo, normalizedPath, ingredient, commit)
		ingredients = append(ingredients, &structs.Ingredient{
			Identifier:    lowerBookID,
			Title:         getSBLocalizedBookTitle(sbMetadata.LocalizedNames, bookID, lowerBookID),
			Path:          normalizedPath,
			Sort:          dcs.GetBookSort(lowerBookID),
			Versification: "ufw",
			Size:          size,
			IsDir:         isDir,
			Exists:        true,
		})
	}
	return ingredients
}

func getSBOBSTSVIngredients(ctx context.Context, gitRepo *git.Repository, sbMetadata *dcs.SBMetadata100, commit *git.Commit) []*structs.Ingredient {
	obsIngredient, ok := sbMetadata.Ingredients["ingredients/OBS.tsv"]
	if !ok || obsIngredient == nil {
		return nil
	}
	size, isDir := getSBIngredientSize(ctx, gitRepo, "./ingredients/OBS.tsv", obsIngredient, commit)
	return []*structs.Ingredient{
		{
			Identifier: "obs",
			Title:      sbMetadata.Identification.Name.DetermineLocalizedTextToUse(),
			Path:       "./ingredients/OBS.tsv",
			Sort:       0,
			Size:       size,
			IsDir:      isDir,
			Exists:     true,
		},
	}
}

func getSBTranslationAcademyIngredients() []*structs.Ingredient {
	return []*structs.Ingredient{
		{
			Path:       "./ingredients/intro",
			Identifier: "intro",
			Title:      "Introduction to Translation Academy",
			Sort:       0,
			IsDir:      true,
			Exists:     true,
		},
		{
			Path:       "./ingredients/process",
			Identifier: "process",
			Title:      "Process Manual",
			Sort:       1,
			IsDir:      true,
			Exists:     true,
		},
		{
			Path:       "./ingredients/translate",
			Identifier: "translate",
			Title:      "Translation Manual",
			Sort:       2,
			IsDir:      true,
			Exists:     true,
		},
		{
			Path:       "./ingredients/checking",
			Identifier: "checking",
			Title:      "Checking Manual",
			Sort:       3,
			IsDir:      true,
			Exists:     true,
		},
	}
}

// metadataFileError builds the ValidationError stored for a metadata file that exists
// but could not be parsed, or does not describe a resource at all. Such a file is
// treated like a schema-invalid one: the entry is kept so the problem is reported on
// the repo's metadata and health check pages instead of the repo silently vanishing.
func metadataFileError(format string, args ...any) *jsonschema.ValidationError {
	return &jsonschema.ValidationError{Message: fmt.Sprintf(format, args...)}
}

// errNotTcTsManifest reports a manifest.json that parses but is neither a tC nor a tS
// manifest, so the caller can look for an RC manifest.yaml before giving up on it.
var errNotTcTsManifest = errors.New("manifest.json is not a translationCore or translationStudio manifest")

// nestedString returns the string at the given key path of a parsed document, or ""
func nestedString(doc map[string]any, keys ...string) string {
	var value any = doc
	for _, key := range keys {
		m, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		value = m[key]
	}
	str, _ := value.(string)
	return str
}

// GetRCDoor43Metadata populates dm from the commit's manifest.yaml. It returns a git
// not-exist error when the commit has no such file.
func GetRCDoor43Metadata(ctx context.Context, gitRepo *git.Repository, dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	blob, err := commit.GetBlobByPath(ctx, gitRepo, "manifest.yaml")
	if err != nil {
		return err
	}
	buf, err := dcs.ReadFileFromBlob(ctx, blob)
	if err != nil {
		return err
	}
	return populateRCDoor43Metadata(ctx, gitRepo, dm, repo, commit, buf)
}

// populateRCDoor43Metadata fills dm from the content of a manifest.yaml. The file's
// presence makes the entry an "rc" one whatever its content: YAML that can't be parsed
// or that fails the RC 0.2 schema is recorded with its error.
func populateRCDoor43Metadata(ctx context.Context, gitRepo *git.Repository, dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit, buf []byte) error {
	dm.RepoID = repo.ID
	dm.MetadataType = "rc"
	dm.MetadataVersion = dcs.GetDefaultMetadataVersionForType("rc")
	dm.Metadata = nil
	dm.ValidationError = nil

	manifest, err := dcs.ParseYAML(buf)
	if err != nil {
		dm.ValidationError = metadataFileError("manifest.yaml could not be parsed as YAML: %v", err)
		return nil
	}
	dm.Metadata = manifest
	if conformsTo := nestedString(manifest, "dublin_core", "conformsto"); strings.HasPrefix(conformsTo, "rc") {
		dm.MetadataVersion = strings.TrimPrefix(conformsTo, "rc")
	}

	dm.ValidationError, err = dcs.ValidateMapByRC02Schema(manifest)
	if err != nil {
		return err // the schema itself could not be loaded: a server problem, not the repo's
	}
	if dm.ValidationError != nil {
		return nil
	}
	return GetDoor43MetadataFromRCManifest(ctx, gitRepo, dm, manifest, repo, commit)
}

// GetTcOrTsDoor43Metadata populates dm from the commit's manifest.json. It returns a
// git not-exist error when the commit has no such file, and errNotTcTsManifest when
// the file is JSON but not a tC/tS manifest (dm then describes it as invalid).
func GetTcOrTsDoor43Metadata(ctx context.Context, gitRepo *git.Repository, dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	blob, err := commit.GetBlobByPath(ctx, gitRepo, "manifest.json")
	if err != nil {
		return err
	}
	log.Debug("%s/%s (%s): manifest.json exists so might be a tC or tS repo", repo.FullName(), dm.Ref, commit.ID)
	buf, err := dcs.ReadFileFromBlob(ctx, blob)
	if err != nil {
		return err
	}
	return populateTcTsDoor43Metadata(ctx, gitRepo, dm, repo, commit, buf)
}

// populateTcTsDoor43Metadata fills dm from the content of a manifest.json. A file that
// is not JSON, has a field of the wrong type, or names no valid book is recorded as an
// invalid tc/ts entry. A JSON file that is neither tc nor ts is described as invalid
// too, but errNotTcTsManifest is returned so the caller can prefer a manifest.yaml.
func populateTcTsDoor43Metadata(ctx context.Context, gitRepo *git.Repository, dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit, buf []byte) error {
	dm.RepoID = repo.ID
	dm.Metadata = nil
	dm.ValidationError = nil

	manifest, err := dcs.ParseJSON(buf)
	if err != nil {
		dm.MetadataType, dm.MetadataVersion = guessTcTsTypeAndVersion(nil, buf, repo.Name)
		dm.ValidationError = metadataFileError("manifest.json could not be parsed as JSON: %v", err)
		return nil
	}
	dm.Metadata = manifest

	t, err := dcs.ParseTcTsManifest(buf)
	if err != nil {
		dm.MetadataType, dm.MetadataVersion = guessTcTsTypeAndVersion(nil, buf, repo.Name)
		dm.ValidationError = metadataFileError("manifest.json has a field of the wrong type: %v", err)
		return nil
	}
	if t.MetadataType == "" {
		dm.MetadataType, dm.MetadataVersion = guessTcTsTypeAndVersion(t, buf, repo.Name)
		dm.ValidationError = metadataFileError("manifest.json is not a supported manifest: translationCore needs tc_version 7 or later, translationStudio needs package_version 3 or later")
		return errNotTcTsManifest
	}
	dm.MetadataType = t.MetadataType
	dm.MetadataVersion = t.MetadataVersion

	if (t.Project.ID == "" || t.Project.ID == "bible") && t.Type.ID != "" {
		t.Project.ID = t.Type.ID
	}
	if t.Project.ID != "tw" && t.Project.ID != "ta" && !dcs.IsValidBook(t.Project.ID) {
		dm.ValidationError = metadataFileError("manifest.json: project id %q is not a valid book identifier", t.Project.ID)
		return nil
	}

	var bookPath string
	var count int
	var versification string
	if t.MetadataType == "ts" {
		bookPath = "."
		if t.Project.ID != "obs" {
			versification = "ufw"
		}
	} else {
		bookPath = "./" + repo.Name + ".usfm"
		if commit != nil {
			count, _ = GetBookAlignmentCount(ctx, gitRepo, bookPath, commit)
		}
		versification = "ufw"
	}

	dm.Repo = repo
	dm.Subject = t.Subject
	dm.FlavorType = t.FlavorType
	dm.Flavor = t.Flavor
	dm.Title = t.Title
	dm.Abbreviation = strings.ToLower(t.Resource.ID)
	dm.Language = strings.ToLower(t.TargetLanguage.ID) // language codes are always stored lowercase
	dm.LanguageTitle = t.TargetLanguage.Name
	dm.LanguageDirection = t.TargetLanguage.Direction
	dm.LanguageIsGL = dcs.LanguageIsGL(t.TargetLanguage.ID)
	dm.ContentFormat = t.Format
	dm.CheckingLevel = 1
	ingredient := &structs.Ingredient{
		Categories:     dcs.GetBookCategories(t.Project.ID),
		Identifier:     t.Project.ID,
		Title:          t.Project.Name,
		Path:           bookPath,
		Sort:           dcs.GetBookSort(t.Project.ID),
		Versification:  versification,
		AlignmentCount: &count,
	}
	if t.MetadataType == "ts" {
		// ts content lives in the repo root
		ingredient.Exists = true
		ingredient.IsDir = true
	} else if commit != nil {
		if entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, bookPath); err == nil {
			ingredient.Exists = true
			ingredient.IsDir = entry.IsDir()
			ingredient.Size = entry.GetSize(ctx, gitRepo)
		}
	}
	dm.Ingredients = []*structs.Ingredient{ingredient}

	return nil
}

// guessTcTsTypeAndVersion picks tc or ts, and a version, for a manifest.json that did
// not validate. The repo naming convention decides first ("_book" is tc, "_text_" is
// ts); failing that, the version key the file carries (parsed when t is given, else
// sniffed from the raw bytes); failing that, tc. The version is the one the file
// declares for the chosen type when it has one, else the type's default.
func guessTcTsTypeAndVersion(t *structs.TcTsManifest, buf []byte, repoName string) (metadataType, version string) {
	metadataType = dcs.GetTcTsMetadataTypeFromRepoName(repoName)
	if metadataType == "" {
		switch {
		case t != nil && t.TcVersion > 0, bytes.Contains(buf, []byte(`"tc_version"`)):
			metadataType = "tc"
		case t != nil && t.TsVersion > 0, bytes.Contains(buf, []byte(`"package_version"`)):
			metadataType = "ts"
		default:
			metadataType = "tc"
		}
	}
	switch {
	case t != nil && metadataType == "tc" && t.TcVersion > 0:
		version = strconv.Itoa(t.TcVersion)
	case t != nil && metadataType == "ts" && t.TsVersion > 0:
		version = strconv.Itoa(t.TsVersion)
	default:
		version = dcs.GetDefaultMetadataVersionForType(metadataType)
	}
	return metadataType, version
}

// GetSBDoor43Metadata populates dm from the commit's metadata.json. It returns a git
// not-exist error when the commit has no such file.
func GetSBDoor43Metadata(ctx context.Context, gitRepo *git.Repository, dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	blob, err := commit.GetBlobByPath(ctx, gitRepo, "metadata.json")
	if err != nil {
		return err
	}
	buf, err := dcs.ReadFileFromBlob(ctx, blob)
	if err != nil {
		return err
	}
	return populateSBDoor43Metadata(ctx, gitRepo, dm, repo, commit, buf)
}

// populateSBDoor43Metadata fills dm from the content of a metadata.json. The file's
// presence makes the entry an "sb" one whatever its content: JSON that can't be parsed
// or that fails the Scripture Burrito schema is recorded with its error.
func populateSBDoor43Metadata(ctx context.Context, gitRepo *git.Repository, dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit, buf []byte) error {
	dm.RepoID = repo.ID
	dm.MetadataType = "sb"
	dm.MetadataVersion = dcs.GetDefaultMetadataVersionForType("sb")
	dm.Metadata = nil
	dm.ValidationError = nil

	metadata, err := dcs.ParseJSON(buf)
	if err != nil {
		dm.ValidationError = metadataFileError("metadata.json could not be parsed as JSON: %v", err)
		return nil
	}
	dm.Metadata = metadata
	if version := nestedString(metadata, "meta", "version"); version != "" {
		dm.MetadataVersion = version
	}

	dm.ValidationError, err = dcs.ValidateMapBySB100Schema(metadata)
	if err != nil {
		return err // the schema itself could not be loaded: a server problem, not the repo's
	}
	if dm.ValidationError != nil {
		return nil
	}

	sbMetadata, err := dcs.ParseSBMetadata(buf)
	if err != nil {
		// schema-valid, yet a field has a shape the struct can't hold: report it the same way
		dm.ValidationError = metadataFileError("metadata.json has a field of the wrong type: %v", err)
		return nil
	}
	return GetDoor43MetadataFromSBMetadata(ctx, gitRepo, dm, sbMetadata, repo, commit)
}

func processDoor43MetadataForRepoRef(ctx context.Context, repo *repo_model.Repository, ref string) error {
	if repo == nil {
		return errors.New("no repository provided")
	}
	if ref == "" {
		return errors.New("no ref provided")
	}

	if repo.IsArchived || repo.IsEmpty || repo.IsMirror || repo.IsPrivate {
		return errors.New("repo must not be empty, an archive, a mirror or private")
	}

	if err := repo.LoadLatestDMs(ctx); err != nil {
		return err
	}

	dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref)
	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		return err
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

	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		log.Error("OpenRepository Error: %v\n", err)
		return err
	}
	defer gitRepo.Close()

	var commit *git.Commit

	dm.Release, err = repo_model.GetRelease(ctx, repo.ID, ref)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		return err
	}
	if dm.Release != nil {
		if dm.Release.IsDraft {
			return nil
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
		commit, err = gitRepo.GetTagCommit(ctx, ref)
		if err != nil {
			log.Error("GetTagCommit [%s/%s]: %v\n", repo.FullName(), ref, err)
			return err
		}
		dm.CommitSHA = commit.ID.String()
		dm.ReleaseDateUnix = dm.Release.CreatedUnix
	} else if !gitRepo.IsBranchExist(ctx, ref) {
		return fmt.Errorf("ref for repo %s [%d] does not exist: %s", repo.FullName(), repo.ID, ref)
	} else {
		dm.Stage = door43metadata.StageOther
		dm.IsLatestForStage = false
		dm.RefType = "branch"
		commit, err = gitRepo.GetBranchCommit(ctx, ref)
		if err != nil {
			log.Error("GetBranchCommit: %v\n", err)
			return err
		}
		dm.CommitSHA = commit.ID.String()
		dm.ReleaseDateUnix = timeutil.TimeStamp(commit.Author.When.Unix())
	}

	// Decide the metadata type by which file the commit carries, in order of precedence:
	// metadata.json (Scripture Burrito), manifest.json (tC/tS), manifest.yaml (RC). A file
	// that exists but can't be parsed or validated still yields an entry of its type
	// carrying the error; only a commit with none of the files is skipped.
	err = GetSBDoor43Metadata(ctx, gitRepo, dm, repo, commit)
	if err != nil && !git.IsErrNotExist(err) {
		log.Debug("processDoor43MetadataForRef: ERROR! Unable to populate DM for %s/%s/metadata.json for SB: %v\n", repo.FullName(), ref, err)
		return err
	}
	if err != nil {
		err = GetTcOrTsDoor43Metadata(ctx, gitRepo, dm, repo, commit)
		notTcTs := errors.Is(err, errNotTcTsManifest)
		if err != nil && !notTcTs && !git.IsErrNotExist(err) {
			log.Debug("processDoor43MetadataForRef: ERROR! Unable to populate DM for %s/%s/manifest.json for TS or TC: %v\n", repo.FullName(), ref, err)
			return err
		}
		if err != nil {
			// no manifest.json, or one that is neither tc nor ts: look for an RC manifest.yaml
			rcErr := GetRCDoor43Metadata(ctx, gitRepo, dm, repo, commit)
			switch {
			case rcErr == nil:
			case !git.IsErrNotExist(rcErr):
				log.Debug("processDoor43MetadataForRef: ERROR! Unable to populate DM for %s/%s/manifest.yaml for RC: %v\n", repo.FullName(), ref, rcErr)
				return rcErr
			case notTcTs:
				// only the unsupported manifest.json exists: keep the invalid tc/ts entry it produced
			default:
				// Not a resource ref: nothing to record, and not an error either (the all-refs
				// pass would otherwise raise an admin notice for every plain branch).
				log.Debug("processDoor43MetadataForRef: %s/%s is not a SB, TC, TS nor RC repo. Not adding to door43_metadata\n", repo.FullName(), ref)
				return nil
			}
		}
	}

	if dm.ValidationError != nil {
		// An invalid entry never stands for a stage; it is kept only to report the error.
		dm.Stage = door43metadata.StageOther
		dm.IsLatestForStage = false
		// Nothing could be extracted from the file, so carry the repo's known values for
		// the display fields (title, language, subject...) instead of leaving them blank.
		dm.CopyEmptyPropertiesFromRepoDM(ctx)
		log.Debug("%s/%s: %s is not valid: %s", repo.FullName(), ref, dm.MetadataFileName(), dcs.ConvertValidationErrorToString(dm.ValidationError))
	} else {
		log.Debug("%s/%s: %s is valid", repo.FullName(), ref, dm.MetadataFileName())
	}

	if err := dm.DetermineAttachmentFlags(ctx); err != nil {
		log.Error("DetermineAttachmentFlags [%s/%s]: %v", repo.FullName(), ref, err)
	}

	if dm.ID > 0 {
		if err = repo_model.UpdateDoor43Metadata(ctx, dm); err != nil {
			return err
		}
	} else if err = repo_model.InsertDoor43Metadata(ctx, dm); err != nil {
		return err
	}

	// Run the health check for this ref so every branch and tag entry carries its own
	// stored severity and issues (the catalog filters on them).
	door43healthcheck.RunHealthcheck(ctx, dm)

	return nil
}

// UpdateUserMetadata updates the user table with their repo languages, subjects and metadata types
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

	// The owner rollup is deferred out of the per-repo work and run once per distinct
	// owner at the end: it reads every one of the owner's entries, so doing it inside
	// the loop repeated the same scan for each of the owner's repos.
	owners := map[int64]*user_model.User{}
	for _, repo := range repos {
		if err := processDoor43MetadataForRepo(ctx, repo, "", false); err != nil {
			log.Info("Failed to process metadata for repo (%v): %v", repo, err)
			if err = system.CreateRepositoryNotice("Failed to process metadata for repository (%s): %v", repo.FullName(), err); err != nil {
				log.Error("ProcessDoor43MetadataForRepo: %v", err)
			}
		}
		if repo.Owner != nil {
			owners[repo.Owner.ID] = repo.Owner
		}
	}

	for _, owner := range owners {
		if err := processDoor43MetadataForUser(ctx, owner); err != nil {
			log.Info("Failed to process metadata for user (%v): %v", owner, err)
			if err = system.CreateRepositoryNotice("Failed to process metadata for user (%s): %v", owner.Name, err); err != nil {
				log.Error("processDoor43MetadataForUser: %v", err)
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

// UpdateDoor43MetadataAttachmentFlags recomputes and saves the has_audio /
// has_video / has_pdf / has_stream / has_other flags on the Door43Metadata of
// the given release after its attachments change. It is a no-op when the
// release has no Door43Metadata entry.
func UpdateDoor43MetadataAttachmentFlags(ctx context.Context, repoID, releaseID int64) error {
	dm, err := repo_model.GetDoor43MetadataByRepoIDAndReleaseID(ctx, repoID, releaseID)
	if err != nil {
		if repo_model.IsErrDoor43MetadataNotExist(err) {
			return nil
		}
		return err
	}
	if err := dm.DetermineAttachmentFlags(ctx); err != nil {
		return err
	}
	if err := repo_model.UpdateDoor43MetadataCols(ctx, dm, repo_model.Door43MetadataAttachmentFlagCols...); err != nil {
		return err
	}
	// An attachment change on any release can change what the repo as a whole has
	return repo_model.AggregateMediaFlagsForRepo(ctx, repoID)
}

// UnpackJSONAttachments expands a release's files.json / links.json manifest
// attachments into one release attachment per entry, each pointing at a remote
// URL (e.g. a YouTube playlist or a file in cloud storage) rather than an
// uploaded blob. The manifest attachment is deleted after a successful
// expansion. Each entry should supply both a "name" and a "browser_download_url";
// an entry without a name falls back to path.Base of the URL path.
// See docs/dcs/remote-release-attachments.md.
func UnpackJSONAttachments(ctx context.Context, release *repo_model.Release) {
	if release == nil || len(release.Attachments) == 0 {
		return
	}
	for _, attachment := range release.Attachments {
		if dcs.IsJSONManifestAttachmentName(attachment.Name) {
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

// GetAttachmentsFromJSON fetches a files.json / links.json manifest attachment
// over HTTP and unmarshals it into attachments. It accepts either a JSON array
// of attachment objects or a single attachment object.
func GetAttachmentsFromJSON(attachment *repo_model.Attachment) ([]*repo_model.Attachment, error) {
	var url string
	if setting.Attachment.Storage.MinioConfig.ServeDirect {
		// If we have a signed url (S3, object storage), redirect to this directly.
		urlObj, err := storage.Attachments.ServeDirectURL(attachment.RelativePath(), attachment.Name, "", nil)

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
	if res.StatusCode != http.StatusOK {
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
