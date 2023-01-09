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
		if err = ProcessDoor43MetadataForRepoRelease(ctx, repo, release); err != nil {
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
		if err = ProcessDoor43MetadataForRepoRelease(ctx, repo, release); err != nil {
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

// ProcessDoor43MetadataForRepoRelease handles the metadata for a given repo by release based on if the container is a valid RC or not
func ProcessDoor43MetadataForRepoRelease(ctx context.Context, repo *repo_model.Repository, release *repo_model.Release) error {
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
	var branchOrTag string
	var commitID string

	if release == nil {
		branchOrTag = repo.DefaultBranch
		stage = door43metadata.StageLatest
		commitID, err = gitRepo.GetBranchCommitID(branchOrTag)
		if err != nil {
			log.Error("GetBranchCommitID: %v\n", err)
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
			branchOrTag = release.TagName
			releaseDateUnix = release.CreatedUnix
			commitID, err = gitRepo.GetTagCommitID(branchOrTag)
			if err != nil {
				log.Error("GetBranchCommitID: %v\n", err)
			}
		} else {
			branchOrTag = release.Target
			releaseDateUnix = timeutil.TimeStamp(commit.Author.When.Unix())
			commitID, err = gitRepo.GetBranchCommitID(branchOrTag)
			if err != nil {
				log.Error("GetBranchCommitID: %v\n", err)
			}
		}
	}

	commit, err = gitRepo.GetBranchCommit(branchOrTag)
	if err != nil {
		log.Error("GetBranchCommit: %v\n", err)
		return err
	}

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
	var projects []*structs.Door43MetadataProject
	var manifest *map[string]interface{}

	blob, err := commit.GetBlobByPath("manifest.yaml")
	if err != nil && !git.IsErrNotExist(err) {
		return err
	}
	if blob != nil { // We have a RC manifest.yaml file, or so we assume
		manifest, err = base.ReadYAMLFromBlob(blob)
		if err != nil {
			return err
		}
		validationResult, err := base.ValidateBlobByRC020Schema(manifest)
		if err != nil {
			return err
		}
		if validationResult != nil {
			log.Warn("%s/%s: manifest.yaml is not valid. see errors:", repo.FullName(), branchOrTag)
			log.Warn("REPO ID: %d, RELEASE ID: %d", repo.ID, releaseID)
			if release != nil {
				log.Warn("RELEASE: %v", release.TagName)
			} else {
				log.Warn("BRANCH: %s", repo.DefaultBranch)
			}
			log.Warn(base.ConvertValidationErrorToString(validationResult))
			return fmt.Errorf("manifest.yaml is not valid")
		}
		log.Info("%s/%s: manifest.yaml is valid.", repo.FullName(), branchOrTag)
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
			bookPath = prod.(map[string]interface{})["path"].(string)
			project := structs.Door43MetadataProject{
				Identifier: prod.(map[string]interface{})["identifier"].(string),
				Title:      prod.(map[string]interface{})["title"].(string),
				Path:       bookPath,
			}
			if subject == "Aligned Bible" && strings.HasSuffix(bookPath, ".usfm") {
				count, _ := GetBookAlignmentCount(bookPath, commit)
				project.AlignmentCount = &count
			}
			projects = append(projects, &project)
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
				checkingLevel, err = strconv.Atoi(cL)
				if err != nil {
					checkingLevel = 1
				}
			}
		}
	} else {
		blob, err := commit.GetBlobByPath("manifest.json")
		if err != nil && !git.IsErrNotExist(err) {
			return err
		}
		if blob == nil {
			return fmt.Errorf("invalid repo: %s: not a rc, tc, or ts repo", repo.FullName())
		}
		log.Info("%s: manifest.json exists so might be a tS or tC repo", repo.FullName())
		tcTsManifest, err := base.ReadTcTsManifestFromBlob(blob)
		if err != nil || tcTsManifest == nil {
			return fmt.Errorf("tried to process a tS or tC repo but manifest.json file invalid")
		}
		if !dcs.BookIsValid(*tcTsManifest.Project.ID) {
			return fmt.Errorf("%s does not have a valid book in its manifest.json", repo.FullName())
		}
		manifest, err = base.ReadJSONFromBlob(blob)
		if err != nil {
			return err
		}
		if tcTsManifest.TcVersion != nil {
			metadataType = "tc"
			metadataVersion = strconv.Itoa(*tcTsManifest.TcVersion)
			subject = "Aligned Bible"
			contentFormat = "usfm"
			bookPath := "./" + repo.Name + ".usfm"
			count, _ := GetBookAlignmentCount(bookPath, commit)
			projects = []*structs.Door43MetadataProject{{
				Identifier:     *tcTsManifest.Project.ID,
				Title:          *tcTsManifest.Project.Name,
				Path:           bookPath,
				AlignmentCount: &count,
			}}
		} else {
			metadataType = "ts"
			metadataVersion = strconv.Itoa(*tcTsManifest.TsVersion)
			contentFormat = "text"
			if (*manifest)["project"].(map[string]string)["id"] == "obs" {
				subject = "TS Open Bible Stories"
			} else {
				subject = "TS Bible"
			}
			projects = []*structs.Door43MetadataProject{{
				Identifier: *tcTsManifest.Project.ID,
				Title:      *tcTsManifest.Project.Name,
				Path:       ".",
			}}
		}
		resource = *tcTsManifest.Resource.ID
		title = *tcTsManifest.Resource.Name
		language = *tcTsManifest.TargetLanguage.ID
		languageTitle = *tcTsManifest.TargetLanguage.Name
		languageDirection = *tcTsManifest.TargetLanguage.Direction
		languageIsGL = dcs.LanguageIsGL(language)
		tCtSJsonBytes, err := json.Marshal(tcTsManifest)
		checkingLevel = 1
		if err != nil {
			return fmt.Errorf("%s: error marshaling TcTsManifest to bytes", repo.FullName())
		}
		err = json.Unmarshal(tCtSJsonBytes, &manifest)
		if err != nil {
			return fmt.Errorf("%s: error Unmarshaling TcTsManifest to generic map", repo.FullName())
		}
	}

	if checkingLevel < 1 || checkingLevel > 3 {
		checkingLevel = 1
	}

	dm, err := repo_model.GetDoor43MetadataByRepoIDAndReleaseID(repo.ID, releaseID)
	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		return err
	}
	if dm != nil {
		err = repo_model.DeleteDoor43Metadata(dm)
		if err != nil {
			return err
		}
	}

	dm = &repo_model.Door43Metadata{
		RepoID:            repo.ID,
		Repo:              repo,
		ReleaseID:         releaseID,
		Release:           release,
		Stage:             stage,
		CommitID:          commitID,
		BranchOrTag:       branchOrTag,
		ReleaseDateUnix:   releaseDateUnix,
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
		Projects:          projects,
		Metadata:          manifest,
	}

	return repo_model.InsertDoor43Metadata(dm)
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
