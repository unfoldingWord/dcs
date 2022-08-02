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
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
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
		releaseID := int64(v)
		v, _ = strconv.ParseInt(string(record["repo_id"]), 10, 64)
		repoID := int64(v)
		if cacheRepos[repoID] == nil {
			cacheRepos[repoID], err = repo_model.GetRepositoryByID(repoID)
			if err != nil {
				log.Warn("GetRepositoryByID Error: %v\n", err)
				continue
			}
		}
		repo := cacheRepos[repoID]
		var release *models.Release
		if releaseID > 0 {
			release, err = models.GetReleaseByID(ctx, releaseID)
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
		_, err := models.DeleteAllDoor43MetadatasByRepoID(repo.ID)
		if err != nil {
			log.Error("DeleteAllDoor43MetadatasByRepoID: %v", err)
		}
		return err
	}

	relIDs, err := models.GetRepoReleaseIDsForMetadata(repo.ID)
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
		var release *models.Release
		releaseRef := repo.DefaultBranch
		if releaseID > 0 {
			release, err = models.GetReleaseByID(ctx, releaseID)
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

// GetAlignmentsCounts get all the alignment counts for all books
func GetAlignmentsCounts(manifest *map[string]interface{}, commit *git.Commit) map[string]int {
	counts := map[string]int{}
	if (*manifest)["dublin_core"].(map[string]interface{})["subject"].(string) != "Aligned Bible" || len((*manifest)["projects"].([]interface{})) == 0 {
		return counts
	}
	for _, prod := range (*manifest)["projects"].([]interface{}) {
		bookPath := prod.(map[string]interface{})["path"].(string)
		if strings.HasSuffix(bookPath, ".usfm") {
			count, _ := GetBookAlignmentCount(bookPath, commit)
			counts[prod.(map[string]interface{})["identifier"].(string)] = count
		}
	}
	return counts
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
func ProcessDoor43MetadataForRepoRelease(ctx context.Context, repo *repo_model.Repository, release *models.Release) error {
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

	var commit *git.Commit
	if release == nil {
		commit, err = gitRepo.GetBranchCommit(repo.DefaultBranch)
		if err != nil {
			log.Error("GetBranchCommit: %v\n", err)
			return err
		}
	} else if !release.IsDraft {
		commit, err = gitRepo.GetTagCommit(release.TagName)
		if err != nil {
			log.Error("GetTagCommit: %v\n", err)
			return err
		}
	} else {
		commit, err = gitRepo.GetBranchCommit(release.Target)
		if err != nil {
			log.Error("GetBranchCommit: %v\n", err)
			return err
		}
	}

	if release != nil {
		UnpackJSONAttachments(ctx, release)
	}

	blob, err := commit.GetBlobByPath("manifest.yaml")
	if err != nil && !git.IsErrNotExist(err) {
		return err
	}
	if blob == nil {
		return nil
	}

	manifest, err := base.ReadYAMLFromBlob(blob)
	if err != nil {
		return err
	}

	validationResult, err := base.ValidateBlobByRC020Schema(manifest)
	if err != nil {
		return err
	}

	var releaseID int64
	var stage door43metadata.Stage
	if release != nil {
		releaseID = release.ID
		if release.IsDraft {
			stage = door43metadata.StageDraft
		} else if release.IsPrerelease {
			stage = door43metadata.StagePreProd
		} else {
			stage = door43metadata.StageProd
		}
	} else {
		stage = door43metadata.StageLatest
	}

	dm, err := models.GetDoor43MetadataByRepoIDAndReleaseID(repo.ID, releaseID)
	if err != nil && !models.IsErrDoor43MetadataNotExist(err) {
		return err
	}

	//metadata, err := ConvertGenericMapToRC020Manifest(manifest)
	//if err != nil {
	//	return err
	//}

	var releaseDateUnix timeutil.TimeStamp
	var branchOrTag string
	if release != nil && !release.IsDraft {
		releaseDateUnix = release.CreatedUnix
		branchOrTag = release.TagName
	} else {
		releaseDateUnix = timeutil.TimeStamp(commit.Author.When.Unix())
		if release != nil {
			branchOrTag = release.Target
		} else {
			branchOrTag = repo.DefaultBranch
		}
	}

	if dm == nil ||
		releaseDateUnix != dm.ReleaseDateUnix ||
		dm.Stage != stage ||
		dm.BranchOrTag != branchOrTag ||
		!reflect.DeepEqual(dm.Metadata, manifest) {
		if validationResult != nil {
			log.Warn("%s/%s: manifest.yaml is not valid. see errors:", repo.FullName(), branchOrTag)
			log.Warn("REPO ID: %d, RELEASE ID: %d", repo.ID, releaseID)
			if release != nil {
				log.Warn("RELEASE: %v", release.TagName)
			} else {
				log.Warn("BRANCH: %s", repo.DefaultBranch)
			}
			log.Warn(base.ConvertValidationErrorToString(validationResult))
			if dm != nil {
				return models.DeleteDoor43Metadata(dm)
			}
		} else {
			if (*manifest)["dublin_core"].(map[string]interface{})["subject"].(string) == "Aligned Bible" {
				(*manifest)["alignment_counts"] = GetAlignmentsCounts(manifest, commit)
			}
			(*manifest)["books"] = GetBooks(manifest)
			lc := (*manifest)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string)
			(*manifest)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["is_gl"] = dcs.LanguageIsGL(lc)
			log.Warn("%s/%s: manifest.yaml is valid.", repo.FullName(), branchOrTag)
			if dm == nil {
				dm = &models.Door43Metadata{
					RepoID:          repo.ID,
					Repo:            repo,
					ReleaseID:       releaseID,
					Release:         release,
					ReleaseDateUnix: releaseDateUnix,
					MetadataVersion: "rc0.2",
					Metadata:        manifest,
					Stage:           stage,
					BranchOrTag:     branchOrTag,
				}
				return models.InsertDoor43Metadata(dm)
			}
			dm.Metadata = manifest
			dm.ReleaseDateUnix = releaseDateUnix
			dm.Stage = stage
			dm.BranchOrTag = branchOrTag
			return models.UpdateDoor43MetadataCols(dm, "lang", "metadata", "release_date_unix", "stage", "branch_or_tag")
		}
	}

	return nil
}

func UnpackJSONAttachments(ctx context.Context, release *models.Release) {
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
