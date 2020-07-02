// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43metadata

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/structs"

	"github.com/ghodss/yaml"
	"github.com/mitchellh/mapstructure"
	"github.com/unknwon/com"
	"github.com/xeipuuv/gojsonschema"
	"xorm.io/xorm"
)

// GenerateDoor43Metadata Generate door43 metadata for valid repos not in the door43_metadata table
func GenerateDoor43Metadata(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

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

	cacheRepos := make(map[int64]*models.Repository)

	for _, record := range records {
		releaseID := com.StrTo(record["release_id"]).MustInt64()
		repoID := com.StrTo(record["repo_id"]).MustInt64()
		fmt.Printf("HERE ====> Repo: %d, Release: %d\n", repoID, releaseID)
		if cacheRepos[repoID] == nil {
			cacheRepos[repoID], err = models.GetRepositoryByID(repoID)
			if err != nil {
				fmt.Printf("GetRepositoryByID Error: %v\n", err)
				continue
			}
		}
		repo := cacheRepos[repoID]
		var release *models.Release
		if releaseID > 0 {
			release, err = models.GetReleaseByID(releaseID)
			if err != nil {
				fmt.Printf("GetReleaseByID Error: %v\n", err)
				continue
			}
		}
		if err = ProcessDoor43MetadataForRepoRelease(repo, release); err != nil {
			continue
		}
	}

	return nil
}

var rc02Schema []byte

// GetRC020Schema Returns the schema for RC v0.2, retrieving it from file if not already done
func GetRC020Schema() ([]byte, error) {
	if rc02Schema == nil {
		var err error
		rc02Schema, err = models.GetRepoInitFile("schema", "rc.schema.json")
		if err != nil {
			return nil, err
		}
	}
	return rc02Schema, nil
}

// ValidateBlobByRC020Schema Validates a blob by the RC v0.2.0 schema and returns the result
func ValidateBlobByRC020Schema(manifest *map[string]interface{}) (*gojsonschema.Result, error) {
	schema, err := GetRC020Schema()
	if err != nil {
		return nil, err
	}
	schemaLoader := gojsonschema.NewBytesLoader(schema)
	documentLoader := gojsonschema.NewGoLoader(manifest)

	return gojsonschema.Validate(schemaLoader, documentLoader)
}

// ReadManifestFromBlob reads a yaml file from a blob and unmarshals it
func ReadManifestFromBlob(blob *git.Blob) (*map[string]interface{}, error) {
	dataRc, err := blob.DataAsync()
	if err != nil {
		fmt.Printf("DataAsync Error: %v\n", err)
		return nil, err
	}
	defer dataRc.Close()
	content, _ := ioutil.ReadAll(dataRc)
	//fmt.Printf("content: %s", content)

	var manifest *map[string]interface{}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		fmt.Printf("yaml.Unmarshal Error: %v", err)
		return nil, err
	}
	return manifest, nil
}

// ConvertGenericMapToRC020Manifest converts a generic map to a RC020Manifest object
func ConvertGenericMapToRC020Manifest(manifest *map[string]interface{}) (*structs.RC020Manifest, error) {
	var rc020manifest structs.RC020Manifest
	err := mapstructure.Decode(*manifest, &rc020manifest)

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

// ProcessDoor43MetadataForRepoRelease handles the metadata for a given repo by release based on if the container is a valid RC or not
func ProcessDoor43MetadataForRepoRelease(repo *models.Repository, release *models.Release) error {
	if repo == nil {
		return fmt.Errorf("no repository provided")
	}
	if release != nil && release.IsTag {
		return fmt.Errorf("release can only be a release, not a tag")
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		fmt.Printf("OpenRepository Error: %v\n", err)
		return err
	}
	defer gitRepo.Close()

	var commit *git.Commit
	if release == nil {
		commit, err = gitRepo.GetBranchCommit(repo.DefaultBranch)
		if err != nil {
			fmt.Printf("GetBranchCommit Error: %v\n", err)
			return err
		}
	} else {
		commit, err = gitRepo.GetTagCommit(release.TagName)
		if err != nil {
			fmt.Printf("GetTagCommit Error: %v\n", err)
			return err
		}
	}

	blob, err := commit.GetBlobByPath("manifest.yaml")
	if blob == nil {
		return nil
	}

	manifest, err := ReadManifestFromBlob(blob)
	if err != nil {
		return err
	}

	result, err := ValidateBlobByRC020Schema(manifest)
	if err != nil {
		return err
	}

	var releaseID int64
	if release != nil {
		releaseID = release.ID
	}

	dm, err := models.GetDoor43MetadataByRepoIDAndReleaseID(repo.ID, releaseID)
	if err != nil && !models.IsErrDoor43MetadataNotExist(err) {
		return err
	}

	//metadata, err := ConvertGenericMapToRC020Manifest(manifest)
	//if err != nil {
	//	return err
	//}

	if result.Valid() {
		fmt.Printf("The document is valid\n")
		if dm == nil {
			dm = &models.Door43Metadata{
				RepoID:          repo.ID,
				ReleaseID:       releaseID,
				MetadataVersion: "rc0.2",
				Metadata:        manifest,
			}
			return models.InsertDoor43Metadata(dm)
		} else if reflect.DeepEqual(dm.Metadata, manifest) {
			dm.Metadata = manifest
			return models.UpdateDoor43MetadataCols(dm, "metadata")
		}
	}

	fmt.Printf("==========\nREPO: %s\n", repo.Name)
	fmt.Printf("REPO ID: %d, RELEASE ID: %d\n", repo.ID, releaseID)
	fmt.Printf("The document is not valid. see errors :\n")
	if release != nil {
		fmt.Printf("RELEASE: %v\n", release.TagName)
	} else {
		fmt.Printf("BRANCH: %s\n", repo.DefaultBranch)
	}
	for _, desc := range result.Errors() {
		fmt.Printf("- %s\n", desc.Description())
		fmt.Printf("- %s = %s\n", desc.Field(), desc.Value())
	}
	if dm != nil {
		return models.DeleteDoor43MetadataByID(dm.ID)
	}
	return nil
}

// ValidateManifestTreeEntry validates a tree entry that is a manifest file and returns the results
func ValidateManifestTreeEntry(entry *git.TreeEntry) (*gojsonschema.Result, error) {
	manifest, err := ReadManifestFromBlob(entry.Blob())
	if err != nil {
		return nil, err
	}
	return ValidateBlobByRC020Schema(manifest)
}

// StringifyValidationErrors returns a semi-colon & new line separated string of the errors
func StringifyValidationErrors(result *gojsonschema.Result) string {
	if result.Valid() {
		return ""
	}
	errStrings := make([]string, len(result.Errors()))
	for i, v := range result.Errors() {
		errStrings[i] = v.String()
	}
	return " * " + strings.Join(errStrings, ";\n * ")
}
