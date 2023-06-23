// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/base"
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
	"github.com/mitchellh/mapstructure"
	"xorm.io/xorm"
)

// GenerateDoor43Metadata Generate door43 metadata for valid repos not in the door43_metadata table
func GenerateDoor43Metadata(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	ctx, commiter, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer commiter.Close()

	// Query to find repos that need processing.
	repos, err := repo_model.GetReposForMetadata(ctx)
	if err != nil {
		return err
	}
	for _, repo := range repos {
		if repo.MetadataUpdatedUnix.AddDuration(24*time.Hour) <= timeutil.TimeStampNow() {
			err := ProcessDoor43MetadataForRepo(ctx, repo, true)
			if err != nil {
				log.Warn("ProcessDoor43MetadataForRepo Error: %v\n", err)
			}
		}
	}
	return nil
}

// ConvertGenericMapToRC020Manifest converts a generic map to a RC020Manifest object
func ConvertGenericMapToRC020Manifest(manifest *map[string]interface{}) (*structs.RC020Manifest, error) {
	var rc020manifest structs.RC020Manifest
	err := mapstructure.Decode(*manifest, &rc020manifest)
	if err != nil {
		return nil, err
	}

	type Checking struct {
		CheckingLevel string `mapstructure:"checking_level"`
	}
	type Language struct {
		Identifier    string
		LangDirection string `mapstructure:"lang_direction"`
	}
	type DublinCore struct {
		Subject  string
		Language Language
		TestThis string `mapstructure:"test_this"`
	}
	type Project struct {
		Identifier string
	}
	type Person struct {
		Checking
		DublinCore `mapstructure:"dublin_core"`
		Projects   []Project
	}

	book1 := map[string]interface{}{"identifier": "gen"}
	book2 := map[string]interface{}{"identifier": "exo"}
	input := map[string]interface{}{
		"dublin_core": map[string]interface{}{
			"subject": "test",
			"language": map[string]interface{}{
				"identifier":     "en",
				"lang_direction": "ltr",
			},
			"test_this": "ok",
		},
		"checking": map[string]interface{}{"checking_level": "1"},
		"projects": []map[string]interface{}{book1, book2},
	}

	var result Person
	err = mapstructure.Decode(input, &result)
	if err != nil {
		panic(err)
	}

	return &rc020manifest, err
}

// ProcessDoor43MetadataForRepoRefs processes the door43 metadata for all the refs of a repo
func ProcessDoor43MetadataForRepoRefs(ctx context.Context, repo *repo_model.Repository) error {
	refs, err := repo_model.GetRepoReleaseTagsForMetadata(ctx, repo.ID)
	if err != nil {
		log.Error("GetRepoReleaseTagsForMetadata Error %s: %v", repo.FullName(), err)
	}

	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		log.Warn("git.OpenRepository Error %s: %v", repo.FullName(), err)
	}
	if gitRepo != nil {
		defer gitRepo.Close()
		branchNames, _, err := gitRepo.GetBranchNames(0, 0)
		if err != nil {
			log.Warn("git.GetBranchNames Error %s: %v", repo.FullName(), err)
		} else {
			refs = append(refs, branchNames...)
		}
	}

	for _, ref := range refs {
		if err := ProcessDoor43MetadataForRef(ctx, repo, ref); err != nil {
			log.Warn("Failed to process metadata for repo %s, ref %s: %v", repo.FullName(), ref, err)
			if err = system.CreateRepositoryNotice("Failed to process metadata for repository (%s) ref (%s): %v", repo.FullName(), ref, err); err != nil {
				log.Error("ProcessDoor43MetadataForRef: %v", err)
			}
		}
	}
	return nil
}

// ProcessDoor43MetadataForRepoLatestDMs determines the latest DMs for a repo
func ProcessDoor43MetadataForRepoLatestDMs(ctx context.Context, repo *repo_model.Repository) error {
	cols := []string{}
	repo.GetLatestProdDm()
	repo.GetLatestPreprodDm()
	repo.GetDefaultBranchDm()
	repo.GetRepoDm()

	// Handle Default Branch DM
	if repo.DefaultBranchDm == nil || repo.DefaultBranchDm.Ref != repo.DefaultBranch {
		dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, repo.DefaultBranch)
		if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
			return err
		}
		if dm != nil && dm.ID != repo.DefaultBranchDmID {
			repo.DefaultBranchDmID = dm.ID
			cols = append(cols, "default_branch_dm_id")
			if dm.Stage != door43metadata.StageLatest {
				dm.Stage = door43metadata.StageLatest
				err = repo_model.UpdateDoor43MetadataCols(ctx, dm, "stage")
				if err != nil {
					return err
				}
			}
			if repo.DefaultBranchDm != nil && repo.DefaultBranchDm.Stage != door43metadata.StageBranch {
				repo.DefaultBranchDm.Stage = door43metadata.StageBranch
				cols = append(cols, "default_branch_dm_id")
				err = repo_model.UpdateDoor43MetadataCols(ctx, dm, "stage")
				if err != nil {
					return err
				}
				repo.DefaultBranchDm = dm
			}
		} else {
			repo.DefaultBranchDmID = 0
			repo.DefaultBranchDm = nil
		}
	}

	// Handle Latest Prod DM
	prodDm, err := repo_model.GetMostRecentDoor43MetadataByStage(ctx, repo.ID, door43metadata.StageProd)
	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		return err
	}
	if prodDm == nil {
		if repo.LatestProdDmID != 0 {
			repo.LatestProdDmID = 0
			repo.LatestProdDm = nil
			cols = append(cols, "latest_prod_dm_id")
		}
	} else if prodDm == nil || prodDm.ID != repo.LatestProdDmID {
		repo.LatestProdDmID = prodDm.ID
		repo.LatestProdDm = prodDm
		cols = append(cols, "latest_prod_dm_id")
	}

	// Handle Latest Preprod DM
	preprodDm, err := repo_model.GetMostRecentDoor43MetadataByStage(ctx, repo.ID, door43metadata.StagePreProd)
	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		return err
	}
	if repo.LatestProdDm != nil && preprodDm != nil && repo.LatestProdDm.ReleaseDateUnix >= preprodDm.ReleaseDateUnix {
		preprodDm = nil
	}
	if preprodDm == nil {
		if repo.LatestPreprodDmID != 0 {
			repo.LatestPreprodDmID = 0
			repo.LatestPreprodDm = nil
			cols = append(cols, "latest_preprod_dm_id")
		}
	} else if repo.LatestPreprodDm == nil || preprodDm.ID != repo.LatestPreprodDm.RepoID {
		repo.LatestPreprodDmID = preprodDm.ID
		repo.LatestPreprodDm = preprodDm
		cols = append(cols, "latest_preprod_dm_id")
	}

	if len(cols) > 0 {
		repo.MetadataUpdatedUnix = timeutil.TimeStampNow()
		cols = append(cols, "metadata_updated_unix")
		err = repo_model.UpdateRepositoryCols(ctx, repo, cols...)
		if err != nil {
			return err
		}
	}

	return nil
}

// ProcessRepoMetadata process the metadata for a repo itself, either DefaultBranchDM or another branch
func ProcessRepoMetadata(ctx context.Context, repo *repo_model.Repository) error {
	cols := []string{}
	if repo.GetRepoDm() == nil || (repo.GetDefaultBranchDm() != nil && repo.RepoDmID != repo.DefaultBranchDmID) {
		repo.RepoDm = repo.DefaultBranchDm
		if repo.RepoDm == nil {
			if repo.RepoDmID == 0 || repo.Subject == "" {
				dm, err := repo_model.GetMostRecentDoor43MetadataByStage(ctx, repo.ID, door43metadata.StageBranch)
				if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
					return err
				}
				repo.RepoDm = dm
			}
		}
		if repo.RepoDm == nil {
			return nil
		}
		repo.RepoDmID = repo.RepoDm.ID
		cols = append(cols, "repo_dm_id")
	}

	dm := repo.RepoDm
	if repo.Subject != dm.Subject {
		repo.Subject = dm.Subject
		cols = append(cols, "subject")
	}
	if repo.Resource != dm.Resource {
		repo.Resource = dm.Resource
		cols = append(cols, "resource")
	}
	if repo.Title != dm.Title {
		repo.Title = dm.Title
		cols = append(cols, "title")
	}
	if repo.MetadataType != dm.MetadataType {
		repo.MetadataType = dm.MetadataType
		cols = append(cols, "metadata_type")
	}
	if repo.MetadataVersion != dm.MetadataVersion {
		repo.MetadataVersion = dm.MetadataVersion
		cols = append(cols, "metadata_version")
	}
	if repo.Language != dm.Language {
		repo.Language = dm.Language
		cols = append(cols, "language")
	}
	if repo.LanguageTitle != dm.LanguageTitle {
		repo.LanguageTitle = dm.LanguageTitle
		cols = append(cols, "language_title")
	}
	if repo.LanguageDirection != dm.LanguageDirection {
		repo.LanguageDirection = dm.LanguageDirection
		cols = append(cols, "language_direction")
	}
	if repo.LanguageIsGL != dm.LanguageIsGL {
		repo.LanguageIsGL = dm.LanguageIsGL
		cols = append(cols, "language_is_gl")
	}
	if repo.ContentFormat != dm.ContentFormat {
		repo.ContentFormat = dm.ContentFormat
		cols = append(cols, "content_format")
	}
	if repo.CheckingLevel != dm.CheckingLevel {
		repo.CheckingLevel = dm.CheckingLevel
		cols = append(cols, "checking_level")
	}
	if !reflect.DeepEqual(repo.Ingredients, dm.Ingredients) {
		repo.Ingredients = dm.Ingredients
		cols = append(cols, "ingredients")
	}

	if len(cols) > 0 {
		if err := repo_model.UpdateRepositoryCols(ctx, repo, cols...); err != nil {
			return err
		}
	}
	return nil
}

// ProcessDoor43MetadataForRepo handles the metadata for a given repo for all its releases
func ProcessDoor43MetadataForRepo(ctx context.Context, repo *repo_model.Repository, processRefs bool) error {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}

	if repo.IsArchived || repo.IsPrivate {
		repo.LatestProdDmID = 0
		repo.LatestPreprodDmID = 0
		repo.DefaultBranchDmID = 0
		repo.MetadataUpdatedUnix = timeutil.TimeStampNow()
		err := repo_model.UpdateRepositoryCols(ctx, repo, "latest_prod_dm_id", "latest_preprod_dm_id", "default_branch_dm_id")
		if err != nil {
			log.Error("UpdateRepositoryCols: %v", err)
		}
		_, err = repo_model.DeleteAllDoor43MetadatasByRepoID(ctx, repo.ID)
		if err != nil {
			log.Error("DeleteAllDoor43MetadatasByRepoID: %v", err)
		}
		return err // No need to process any thing else below
	}

	if processRefs {
		if err := ProcessDoor43MetadataForRepoRefs(ctx, repo); err != nil {
			// log error but keep on going
			log.Error("ProcessDoor43MetadataForRepoRefs %s Error: %v", repo.FullName(), err)
		}
	}

	if err := ProcessDoor43MetadataForRepoLatestDMs(ctx, repo); err != nil {
		log.Error("ProcessDoor43MetadataForRepoLatestDMs %s Error: %v", repo.FullName(), err)
	}

	if err := ProcessRepoMetadata(ctx, repo); err != nil {
		log.Error("ProcessRepoMetadata %s Error: %v", repo.FullName(), err)
	}

	return nil
}

func GetBookAlignmentCount(bookPath string, commit *git.Commit) (int, error) {
	blob, err := commit.GetBlobByPath(bookPath)
	if err != nil {
		log.Warn("GetBlobByPath(%s) Error: %v\n", bookPath, err)
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
func GetBooks(manifest *map[string]interface{}) []string {
	var books []string
	if len((*manifest)["projects"].([]interface{})) > 0 {
		for _, prod := range (*manifest)["projects"].([]interface{}) {
			books = append(books, prod.(map[string]interface{})["identifier"].(string))
		}
	}
	return books
}

func GetDoor43MetadataFromRCManifest(dm *repo_model.Door43Metadata, manifest *map[string]interface{}, repo *repo_model.Repository, commit *git.Commit) error {
	var metadataType string
	var metadataVersion string
	var subject string
	var resource string
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
	matches := re.FindStringSubmatch((*manifest)["dublin_core"].(map[string]interface{})["conformsto"].(string))
	if len(matches) == 3 {
		metadataType = matches[1]
		metadataVersion = matches[2]
	} else {
		// should never get here since schema validated
		metadataType = "rc"
		metadataVersion = "0.2"
	}
	subject = (*manifest)["dublin_core"].(map[string]interface{})["subject"].(string)
	resource = (*manifest)["dublin_core"].(map[string]interface{})["identifier"].(string)
	title = (*manifest)["dublin_core"].(map[string]interface{})["title"].(string)
	language = (*manifest)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string)
	languageTitle = (*manifest)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["title"].(string)
	format = (*manifest)["dublin_core"].(map[string]interface{})["format"].(string)
	languageDirection = dcs.GetLanguageDirection(language)
	languageIsGL = dcs.LanguageIsGL(language)
	var bookPath string

	for _, prod := range (*manifest)["projects"].([]interface{}) {
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
	} else if strings.HasPrefix(subject, "TSV ") {
		if strings.HasPrefix(fmt.Sprintf("./%s_", resource), bookPath) {
			contentFormat = "tsv7"
		} else {
			contentFormat = "tsv9"
		}
	} else if strings.Contains(format, "/") {
		contentFormat = strings.Split(format, "/")[1]
	} else if repo.PrimaryLanguage != nil {
		contentFormat = strings.ToLower(repo.PrimaryLanguage.Language)
	} else {
		contentFormat = "markdown"
	}
	var ok bool
	checkingLevel, ok = (*manifest)["checking"].(map[string]interface{})["checking_level"].(int)
	if !ok {
		cL, ok := (*manifest)["checking"].(map[string]interface{})["checking_level"].(string)
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
	dm.Title = title
	dm.Resource = resource
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

func GetNewDoor43MetadataFromSBData(dm *repo_model.Door43Metadata, sbData *base.SB100, repo *repo_model.Repository, commit *git.Commit) error {
	var metadataType string
	var metadataVersion string
	subject := "unknown"
	var resource string
	var title string
	var language string
	var languageTitle string
	var languageDirection string
	var languageIsGL bool
	var contentFormat string
	checkingLevel := 1
	var ingredients []*structs.Ingredient

	metadataType = "sb"
	metadataVersion = sbData.Meta.Version
	title = sbData.Identification.Name.En

	switch sbData.Type.FlavorType.Name {
	case "scripture":
		subject = "Bible"
		resource = strings.ToLower(sbData.Identification.Abbreviation.En)
		contentFormat = "usfm"
		if sbData.LocalizedNames != nil {
			for book, ln := range *sbData.LocalizedNames {
				bookPath := "./ingredients/" + book + ".usfm"
				count, _ := GetBookAlignmentCount(bookPath, commit)
				ingredients = append(ingredients, &structs.Ingredient{
					Categories:     dcs.GetBookCategories(book),
					Identifier:     book,
					Title:          ln.Short.En,
					Path:           bookPath,
					Sort:           dcs.GetBookSort(book),
					AlignmentCount: &count,
				})
			}
		}
	case "gloss":
		switch sbData.Type.FlavorType.Flavor.Name {
		case "textStories":
			subject = "Open Bible Stories"
			resource = "obs"
			contentFormat = "markdown"
			ingredients = append(ingredients, &structs.Ingredient{
				Identifier: "obs",
				Title:      title,
				Path:       "./ingredients",
			})
		}
	}

	if len(sbData.Languages) > 0 {
		language = sbData.Languages[0].Tag
		languageTitle = dcs.GetLanguageTitle(language)
		if languageTitle == "" {
			languageTitle = sbData.Languages[0].Name.En
		}
		languageDirection = dcs.GetLanguageDirection(language)
		languageIsGL = dcs.LanguageIsGL(language)
	}

	dm.RepoID = repo.ID
	dm.MetadataType = metadataType
	dm.MetadataVersion = metadataVersion
	dm.Subject = subject
	dm.Title = title
	dm.Resource = resource
	dm.Language = language
	dm.LanguageTitle = languageTitle
	dm.LanguageDirection = languageDirection
	dm.LanguageIsGL = languageIsGL
	dm.ContentFormat = contentFormat
	dm.CheckingLevel = checkingLevel
	dm.Ingredients = ingredients
	dm.Metadata = sbData.Metadata

	return nil
}

func GetRCDoor43Metadata(dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	var manifest *map[string]interface{}

	blob, err := commit.GetBlobByPath("manifest.yaml")
	if err != nil {
		return err
	}
	if blob == nil {
		return nil
	}
	manifest, err = base.ReadYAMLFromBlob(blob)
	if err != nil {
		return err
	}
	validationResult, err := base.ValidateMapByRC020Schema(manifest)
	if err != nil {
		return err
	}
	if validationResult != nil {
		log.Warn("%s: manifest.yaml is not valid. see errors:", repo.FullName())
		log.Warn(base.ConvertValidationErrorToString(validationResult))
		return fmt.Errorf("manifest.yaml is not valid")
	}
	log.Info("%s: manifest.yaml is valid.", repo.FullName())
	return GetDoor43MetadataFromRCManifest(dm, manifest, repo, commit)
}

func GetTcOrTsDoor43Metadata(dm *repo_model.Door43Metadata, repo *repo_model.Repository, commit *git.Commit) error {
	blob, err := commit.GetBlobByPath("manifest.json")
	if err != nil || blob == nil {
		return err
	}

	log.Info("%s: manifest.json exists so might be a tC or tS repo", repo.FullName())
	var bookPath string
	var contentFormat string
	var count int
	var versification string

	t, err := base.GetTcTsManifestFromBlob(blob)
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

	if !dcs.BookIsValid(t.Project.ID) {
		return fmt.Errorf("%s does not have a valid book in its manifest.json", repo.FullName())
	}

	// Get the manifest again in map[string]interface{} format for the DM object
	manifest, err := base.ReadJSONFromBlob(blob)
	if err != nil {
		return err
	}

	dm.RepoID = repo.ID
	dm.Repo = repo
	dm.MetadataType = t.MetadataType
	dm.MetadataVersion = t.MetadataVersion
	dm.Subject = t.Subject
	dm.Title = t.Title
	dm.Resource = strings.ToLower(t.Resource.ID)
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
	blob, err := commit.GetBlobByPath("metadata.json")
	if err != nil {
		return err
	}
	if blob == nil {
		return nil
	}
	sbData, err := base.GetSBDataFromBlob(blob)
	if err != nil {
		return err
	}
	// validationResult, err := base.ValidateDataBySB100Schema(sbData)
	// if err != nil {
	// 	return nil, err
	// }
	// if validationResult != nil {
	// 	log.Warn("%s: metadata.json's 'data' is not valid. see errors:", repo.FullName())
	// 	log.Warn(base.ConvertValidationErrorToString(validationResult))
	// 	return nil, fmt.Errorf("metadata.json's 'data' is not valid")
	// }
	// log.Info("%s: metadata.json's 'data' is valid.", repo.FullName())

	return GetNewDoor43MetadataFromSBData(dm, sbData, repo, commit)
}

// ProcessDoor43MetadataForRef handles the metadata for a given repo by release or branch (ref) based on if the container is a valid RC or not
func ProcessDoor43MetadataForRef(ctx context.Context, repo *repo_model.Repository, ref string) (err error) {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}
	if ref == "" {
		return fmt.Errorf("no ref profided")
	}

	err = repo.LoadAttributes(ctx)
	if err != nil {
		return err
	}

	var dm *repo_model.Door43Metadata
	dm, err = repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref)
	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		return err
	}
	if dm != nil {
		defer func() {
			if err != nil {
				// There was a problem updating the branch metadata, so we want to invalidated it by setting
				// default_branch_dm_id to 0 if it is this dm.
				if dm != nil && repo.DefaultBranchDmID == dm.ID {
					repo.DefaultBranchDm = nil
					repo.DefaultBranchDmID = 0
					_ = repo_model.UpdateRepositoryCols(ctx, repo, "default_branch_dm_id")
				}
			}
		}()
	} else {
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

	release, err := repo_model.GetRelease(repo.ID, ref)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		return err
	}
	if release != nil {
		if release.IsTag || release.IsDraft {
			return fmt.Errorf("ref for repo %s [%d] must be a branch or a (pre-)release: %s", repo.FullName(), repo.ID, ref)
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
			log.Error("GetTagCommit: %v\n", err)
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
	if err != nil && !git.IsErrNotExist((err)) {
		return err
	}

	// Check for TC or TS
	if err != nil {
		err = GetTcOrTsDoor43Metadata(dm, repo, commit)
		if err != nil && !git.IsErrNotExist(err) {
			return err
		}
	}

	// Check for RC
	if err != nil {
		err = GetRCDoor43Metadata(dm, repo, commit)
		if err != nil {
			if !git.IsErrNotExist(err) {
				log.Error("ProcessDoor43MetadataForRef: ERROR! Unable to populate DM for %s/%s from RC manifest.yaml: %v\n", repo.FullName(), ref, err)
				return err
			}
			log.Info("ProcessDoor43MetadataForRef: %s/%s is not a SB, TC, TS nor RC repo. Not adding to door43_metadata\n", repo.FullName(), ref)
			return nil // nothing to process, not a SB, TC, TS nor RC repo
		}
	}

	dm.CommitSHA = commitID
	dm.ReleaseID = releaseID
	dm.Release = release
	dm.ReleaseDateUnix = releaseDateUnix
	dm.Stage = stage

	if dm.ID > 0 && dm.Stage >= door43metadata.StageLatest {
		defer func() {
			if err != nil {
				// There was a problem updating the branch metadata, so we want to invalidated it by deleting it.
				_ = repo_model.DeleteDoor43Metadata(dm)
				if dm.Stage == door43metadata.StageLatest {
					repo.DefaultBranchDm = nil
					repo.DefaultBranchDmID = 0
					_ = repo_model.UpdateRepositoryCols(ctx, repo, "default_branch_dm_id")
				}
			}
		}()
	}

	if dm.ID > 0 {
		err = repo_model.UpdateDoor43Metadata(dm)
		if err != nil {
			return err
		}
	} else {
		err = repo_model.InsertDoor43Metadata(dm)
		if err != nil {
			return err
		}
	}

	return nil
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
				log.Warn("GetAttachmentsFromJSON Error: %v", err)
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
							log.Warn("UpdateAttachment [%d]: %v", a.ID, err)
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
					log.Warn("insert attachment [%d]: %v", remoteAttachment.ID, err)
					continue
				}
			}
			if err := repo_model.DeleteAttachment(attachment, true); err != nil {
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
