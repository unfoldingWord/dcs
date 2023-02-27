// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43metadata

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"xorm.io/xorm"
)

// GenerateDoor43Metadata Generate door43 metadata for valid repos not in the door43_metadata table
func GenerateDoor43Metadata(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	ctx, commiter, err := db.TxContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	// Query to find repos that need processing, either having releases that
	// haven't been processed, or their default branch hasn't been processed.
	records, err := sess.Query("SELECT rel.id as release_id, r.id as repo_id  FROM `repository` r " +
		"  JOIN `release` rel ON rel.repo_id = r.id " +
		"  LEFT JOIN `door43_metadata` dm ON r.id = dm.repo_id " +
		"  AND rel.id = dm.release_id " +
		"  WHERE dm.id IS NULL AND rel.is_tag = 0 " +
		"UNION " +
		"SELECT 0 as `release_id`, r2.id as repo_id FROM `repository` r2 " +
		"  LEFT JOIN `door43_metadata` dm2 ON r2.id = dm2.repo_id " +
		"  AND dm2.release_id = 0 " +
		"  WHERE dm2.id IS NULL " +
		"ORDER BY repo_id ASC, release_id ASC")
	if err != nil {
		return err
	}

	cacheRepos := make(map[int64]*repo_model.Repository)

	for _, record := range records {
		v, _ := strconv.ParseInt(string(record["release_id"]), 10, 64)
		releaseID := v
		v, _ = strconv.ParseInt(string(record["repo_id"]), 10, 64)
		repoID := v
		if cacheRepos[repoID] == nil {
			cacheRepos[repoID], err = repo_model.GetRepositoryByID(repoID)
			if err != nil {
				log.Warn("GetRepositoryByID Error: %v\n", err)
				continue
			}
		}
		repo := cacheRepos[repoID]
		var release *repo_model.Release
		if releaseID > 0 {
			release, err = repo_model.GetReleaseByID(ctx, releaseID)
			if err != nil {
				log.Warn("GetReleaseByID Error: %v\n", err)
				continue
			}
		}
		if err = ProcessDoor43MetadataForRepoRelease(ctx, repo, release, release.TagName); err != nil {
			continue
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

// ProcessDoor43MetadataForRepo handles the metadata for a given repo for all its releases
func ProcessDoor43MetadataForRepo(repo *repo_model.Repository) error {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}

	if repo.IsArchived || repo.IsPrivate {
		_, err := repo_model.DeleteAllDoor43MetadatasByRepoID(repo.ID)
		if err != nil {
			log.Error("DeleteAllDoor43MetadatasByRepoID: %v", err)
		}
		return err
	}

	relIDs, err := repo_model.GetRepoReleaseIDsForMetadata(repo.ID)
	if err != nil {
		log.Error("GetReleaseIDsNeedingDoor43Metadata: %v", err)
		return err
	}

	ctx, commiter, err := db.TxContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	for _, releaseID := range relIDs {
		var release *repo_model.Release
		releaseRef := repo.DefaultBranch
		if releaseID > 0 {
			release, err = repo_model.GetReleaseByID(ctx, releaseID)
			if err != nil {
				log.Error("GetReleaseByID Error: %v\n", err)
				continue
			}
			releaseRef = release.TagName
		}
		log.Info("Processing Metadata for repo %s (%d), %s (%d)\n", repo.Name, repo.ID, releaseRef, releaseID)
		if err = ProcessDoor43MetadataForRepoRelease(ctx, repo, release, releaseRef); err != nil {
			log.Error("Error processing metadata for repo %s (%d), %s (%d): %v\n", repo.Name, repo.ID, releaseRef, releaseID, err)
		} else {
			log.Info("Processed Metadata for repo %s (%d), %s (%d)\n", repo.Name, repo.ID, releaseRef, releaseID)
		}
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

func GetNewDoor43MetadataFromRCManifest(manifest *map[string]interface{}, repo *repo_model.Repository, commit *git.Commit) (*repo_model.Door43Metadata, error) {
	var metadataType string
	var metadataVersion string
	var subject string
	var resource string
	var title string
	var language string
	var languageTitle string
	var languageDirection string
	var languageIsGL bool
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

	return &repo_model.Door43Metadata{
		RepoID:            repo.ID,
		MetadataType:      metadataType,
		MetadataVersion:   metadataVersion,
		Subject:           subject,
		Title:             title,
		Resource:          resource,
		Language:          language,
		LanguageTitle:     languageTitle,
		LanguageDirection: languageDirection,
		LanguageIsGL:      languageIsGL,
		ContentFormat:     contentFormat,
		CheckingLevel:     checkingLevel,
		Ingredients:       ingredients,
		Metadata:          manifest,
	}, nil
}

func GetNewDoor43MetadataFromSBData(sbData *base.SB100, repo *repo_model.Repository, commit *git.Commit) (*repo_model.Door43Metadata, error) {
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

	return &repo_model.Door43Metadata{
		RepoID:            repo.ID,
		MetadataType:      metadataType,
		MetadataVersion:   metadataVersion,
		Subject:           subject,
		Title:             title,
		Resource:          resource,
		Language:          language,
		LanguageTitle:     languageTitle,
		LanguageDirection: languageDirection,
		LanguageIsGL:      languageIsGL,
		ContentFormat:     contentFormat,
		CheckingLevel:     checkingLevel,
		Ingredients:       ingredients,
		Metadata:          sbData.Metadata,
	}, nil
}

func GetNewRCDoor43Metadata(repo *repo_model.Repository, commit *git.Commit) (*repo_model.Door43Metadata, error) {
	var manifest *map[string]interface{}

	blob, err := commit.GetBlobByPath("manifest.yaml")
	if err != nil {
		return nil, err
	}
	if blob == nil {
		return nil, nil
	}
	manifest, err = base.ReadYAMLFromBlob(blob)
	if err != nil {
		return nil, err
	}
	validationResult, err := base.ValidateMapByRC020Schema(manifest)
	if err != nil {
		return nil, err
	}
	if validationResult != nil {
		log.Warn("%s: manifest.yaml is not valid. see errors:", repo.FullName())
		log.Warn(base.ConvertValidationErrorToString(validationResult))
		return nil, fmt.Errorf("manifest.yaml is not valid")
	}
	log.Info("%s: manifest.yaml is valid.", repo.FullName())
	return GetNewDoor43MetadataFromRCManifest(manifest, repo, commit)
}

func GetNewTcOrTsDoor43Metadata(repo *repo_model.Repository, commit *git.Commit) (*repo_model.Door43Metadata, error) {
	blob, err := commit.GetBlobByPath("manifest.json")
	if err != nil || blob == nil {
		return nil, err
	}

	log.Info("%s: manifest.json exists so might be a tC or tS repo", repo.FullName())
	var bookPath string
	var contentFormat string
	var count int
	var versification string

	t, err := base.GetTcTsManifestFromBlob(blob)
	if err != nil || t == nil {
		return nil, err
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
		return nil, fmt.Errorf("%s does not have a valid book in its manifest.json", repo.FullName())
	}

	// Get the manifest again in map[string]interface{} format for the DM object
	manifest, err := base.ReadJSONFromBlob(blob)
	if err != nil {
		return nil, err
	}

	dm := &repo_model.Door43Metadata{
		RepoID:            repo.ID,
		Repo:              repo,
		MetadataType:      t.MetadataType,
		MetadataVersion:   t.MetadataVersion,
		Subject:           t.Subject,
		Title:             t.Title,
		Resource:          strings.ToLower(t.Resource.ID),
		Language:          t.TargetLanguage.ID,
		LanguageTitle:     t.TargetLanguage.Name,
		LanguageDirection: t.TargetLanguage.Direction,
		LanguageIsGL:      dcs.LanguageIsGL(t.TargetLanguage.ID),
		ContentFormat:     contentFormat,
		CheckingLevel:     1,
		Ingredients: []*structs.Ingredient{{
			Categories:     dcs.GetBookCategories(t.Project.ID),
			Identifier:     t.Project.ID,
			Title:          t.Project.Name,
			Path:           bookPath,
			Sort:           dcs.GetBookSort(t.Project.ID),
			Versification:  versification,
			AlignmentCount: &count,
		}},
		Metadata: manifest,
	}
	return dm, nil
}

func GetNewSBDoor43Metadata(repo *repo_model.Repository, commit *git.Commit) (*repo_model.Door43Metadata, error) {
	blob, err := commit.GetBlobByPath("metadata.json")
	if err != nil {
		return nil, err
	}
	if blob == nil {
		return nil, nil
	}
	sbData, err := base.GetSBDataFromBlob(blob)
	if err != nil {
		return nil, err
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

	return GetNewDoor43MetadataFromSBData(sbData, repo, commit)
}

// ProcessDoor43MetadataForRepoRelease handles the metadata for a given repo by release based on if the container is a valid RC or not
func ProcessDoor43MetadataForRepoRelease(ctx context.Context, repo *repo_model.Repository, release *repo_model.Release, branchOrTag string) (err error) {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}
	if release != nil && release.IsTag {
		return fmt.Errorf("release can only be a release, not a tag")
	}

	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		log.Error("OpenRepository Error: %v\n", err)
		return err
	}
	defer gitRepo.Close()

	var releaseID int64
	var stage door43metadata.Stage
	var commit *git.Commit
	var releaseDateUnix timeutil.TimeStamp
	var commitID string

	if release == nil {
		stage = door43metadata.StageLatest
		commitID, err = gitRepo.GetBranchCommitID(branchOrTag)
		if err != nil {
			log.Error("GetBranchCommitID: %v\n", err)
			return fmt.Errorf("unable to get a branch commit id")
		}
	} else {
		releaseID = release.ID

		if release.IsDraft {
			stage = door43metadata.StageDraft
		} else if release.IsPrerelease {
			stage = door43metadata.StagePreProd
		} else {
			stage = door43metadata.StageProd
		}

		if !release.IsDraft {
			releaseDateUnix = release.CreatedUnix
			commitID, err = gitRepo.GetTagCommitID(branchOrTag)
			if err != nil {
				log.Error("GetTagCommitID: %v\n", err)
			}
		} else {
			branchOrTag = release.Target
			commitID, err = gitRepo.GetBranchCommitID(branchOrTag)
			if err != nil {
				log.Error("GetBranchCommitID: %v\n", err)
			}
		}
	}

	var oldDM *repo_model.Door43Metadata
	if release != nil || branchOrTag == repo.DefaultBranch {
		oldDM, err = repo_model.GetDoor43MetadataByRepoIDAndReleaseID(repo.ID, releaseID)
		if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
			return err
		}
	}
	if oldDM != nil && (oldDM.Stage == door43metadata.StageDraft || oldDM.Stage == door43metadata.StageLatest) {
		defer func() {
			if err != nil {
				// There was a problem updating the draft release or default branch, so we want to invalidated it by deleting it.
				_ = repo_model.DeleteDoor43Metadata(oldDM)
			}
		}()
	}

	commit, err = gitRepo.GetCommit(commitID)
	if err != nil {
		log.Error("GetCommit: %v\n", err)
		return err
	}

	if release == nil || release.IsDraft {
		releaseDateUnix = timeutil.TimeStamp(commit.Author.When.Unix())
	}

	// Check for SB (Scripture Burrito)
	dm, err := GetNewSBDoor43Metadata(repo, commit)
	if err != nil && !git.IsErrNotExist((err)) {
		return err
	}

	// Check for RC (Resource Container)

	// Check for TC or TS
	if dm == nil {
		dm, err = GetNewTcOrTsDoor43Metadata(repo, commit)
		if err != nil && !git.IsErrNotExist(err) {
			return err
		}
	}

	// Check for RC
	if dm == nil {
		dm, err = GetNewRCDoor43Metadata(repo, commit)
		if err != nil {
			if !git.IsErrNotExist(err) {
				return err
			}
			return nil // nothing to process, not a SB, TC, TS nor RC repo
		}
	}

	dm.BranchOrTag = branchOrTag
	dm.CommitID = commitID
	dm.Release = release
	dm.ReleaseID = releaseID
	dm.ReleaseDateUnix = releaseDateUnix
	dm.Stage = stage
	dm.Latest = true // set true by default

	if dm.ReleaseID > 0 {
		latestDMs, err := repo_model.GetLatestDoor43MetadatasByRepoID(repo.ID)
		if err != nil {
			return err
		}
		if len(latestDMs) > 0 {
			for _, latestDM := range latestDMs {
				if latestDM.ReleaseID > 0 {
					if dm.ReleaseDateUnix >= latestDM.ReleaseDateUnix && dm.Stage <= latestDM.Stage {
						latestDM.Latest = false
						err = repo_model.UpdateDoor43MetadataCols(latestDM, "latest")
						if err != nil {
							return err
						}
					}
					if dm.ReleaseDateUnix < latestDM.ReleaseDateUnix && dm.Stage >= latestDM.Stage {
						dm.Latest = false
					}
				}
			}
		}
	}

	if release != nil || branchOrTag == repo.DefaultBranch {
		if oldDM != nil {
			dm.ID = oldDM.ID
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
	}

	// Insert or Update the repo entry of StageRepo (4) and ReleaseID of -1
	if releaseID == 0 {
		dm.ReleaseID = -1 // Need to set this to -1 so it is unique to the master branch, which is 0
		dm.Stage = door43metadata.StageRepo
		dm.Latest = branchOrTag != repo.DefaultBranch
		// master branch was processed, so we make or update another entry with Stage = Repo and releaseID = -1 so we always retain repo metadata if master goes bad
		oldDM, err = repo_model.GetDoor43MetadataByRepoIDAndStage(repo.ID, door43metadata.StageRepo)
		if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
			return err
		}
		if oldDM == nil || oldDM.BranchOrTag == branchOrTag || branchOrTag == repo.DefaultBranch {
			if oldDM != nil {
				dm.ID = oldDM.ID
				return repo_model.UpdateDoor43Metadata(dm)
			}
			dm.ID = 0
			return repo_model.InsertDoor43Metadata(dm)
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
	if setting.Attachment.ServeDirect {
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
