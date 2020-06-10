// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"sort"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/ghodss/yaml"
	"github.com/xeipuuv/gojsonschema"
	"xorm.io/builder"
)

// Door43Metadata represents the metadata of repository's release or default branch (ReleaseID = 0).
type Door43Metadata struct {
	ID              int64                  `xorm:"pk autoincr"`
	RepoID          int64                  `xorm:"INDEX UNIQUE(n) NOT NULL"`
	Repo            *Repository            `xorm:"-"`
	ReleaseID       int64                  `xorm:"INDEX UNIQUE(n)"`
	Release         *Release               `xorm:"-"`
	MetadataVersion string                 `xorm:"NOT NULL"`
	Metadata        map[string]interface{} `xorm:"JSON NOT NULL"`
	CreatedUnix     timeutil.TimeStamp     `xorm:"INDEX created NOT NULL"`
	UpdatedUnix     timeutil.TimeStamp     `xorm:"INDEX updated"`
}

func (dm *Door43Metadata) loadAttributes(e Engine) error {
	var err error
	if dm.Repo == nil {
		dm.Repo, err = GetRepositoryByID(dm.RepoID)
		if err != nil {
			return err
		}
	}
	if dm.Release == nil && dm.ReleaseID > 0 {
		dm.Release, err = GetReleaseByID(dm.ReleaseID)
		if err != nil {
			return err
		}
	}
	return nil
}

// LoadAttributes load repo and release attributes for a door43 metadata
func (dm *Door43Metadata) LoadAttributes() error {
	return dm.loadAttributes(x)
}

// APIURL the api url for a door43 metadata. door43 metadata must have attributes loaded
func (dm *Door43Metadata) APIURL() string {
	return fmt.Sprintf("%sapi/v1/repos/%s/metadata/%d",
		setting.AppURL, dm.Repo.FullName(), dm.ID)
}

// HTMLURL the url for a door43 metadata on the web UI. door43 metadata must have attributes loaded
func (dm *Door43Metadata) HTMLURL() string {
	return fmt.Sprintf("%s/metadata/tag/%s", dm.Repo.HTMLURL(), dm.Release.TagName)
}

// APIFormat convert a Door43Metadata to api.Door43Metadata
func (dm *Door43Metadata) APIFormat() *api.Door43Metadata {
	return &api.Door43Metadata{
		ID:        dm.ID,
		CreatedAt: dm.CreatedUnix.AsTime(),
	}
}

// IsDoor43MetadataExist returns true if door43 metadata with given release ID already exists.
func IsDoor43MetadataExist(repoID, releaseID int64) (bool, error) {
	return x.Get(&Door43Metadata{RepoID: repoID, ReleaseID: releaseID})
}

// InsertDoor43Metadata inserts a door43 metadata
func InsertDoor43Metadata(dm *Door43Metadata) error {
	_, err := x.Insert(dm)
	return err
}

// InsertDoor43MetadatasContext inserts door43 metadatas
func InsertDoor43MetadatasContext(ctx DBContext, dms []*Door43Metadata) error {
	_, err := ctx.e.Insert(dms)
	return err
}

// UpdateDoor43MetadataCols update door43 metadata according special columns
func UpdateDoor43MetadataCols(dm *Door43Metadata, cols ...string) error {
	return updateDoor43MetadataCols(x, dm, cols...)
}

func updateDoor43MetadataCols(e Engine, dm *Door43Metadata, cols ...string) error {
	_, err := e.ID(dm.ID).Cols(cols...).Update(dm)
	return err
}

// GetDoor43Metadata returns metadata by given repo ID and release ID.
func GetDoor43Metadata(repoID, releaseID int64) (*Door43Metadata, error) {
	isExist, err := IsDoor43MetadataExist(repoID, releaseID)
	if err != nil {
		return nil, err
	} else if !isExist {
		return nil, ErrDoor43MetadataNotExist{0, releaseID}
	}

	rel := &Door43Metadata{RepoID: repoID, ReleaseID: releaseID}
	_, err = x.Get(rel)
	return rel, err
}

// GetDoor43MetadataByRepoIDAndTagName returns metadata by given repo ID and tag name.
func GetDoor43MetadataByRepoIDAndTagName(repoID int64, tagName string) (*Door43Metadata, error) {
	var releaseID int64

	if tagName != "" && tagName != "default" {
		release, err := GetRelease(repoID, tagName)
		if err != nil {
			return nil, err
		}
		releaseID = release.ID
	}

	isExist, err := IsDoor43MetadataExist(repoID, releaseID)
	if err != nil {
		return nil, err
	} else if !isExist {
		return nil, ErrDoor43MetadataNotExist{0, releaseID}
	}

	dm := &Door43Metadata{RepoID: repoID, ReleaseID: releaseID}
	_, err = x.Get(dm)
	return dm, err
}

// GetDoor43MetadataByID returns door43 metadata with given ID.
func GetDoor43MetadataByID(id int64) (*Door43Metadata, error) {
	rel := new(Door43Metadata)
	has, err := x.
		ID(id).
		Get(rel)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDoor43MetadataNotExist{id, 0}
	}

	return rel, nil
}

// FindDoor43MetadatasOptions describes the conditions to find door43 metadatas
type FindDoor43MetadatasOptions struct {
	ListOptions
	ReleaseIDs []int64
}

func (opts *FindDoor43MetadatasOptions) toConds(repoID int64) builder.Cond {
	var cond = builder.NewCond()
	cond = cond.And(builder.Eq{"repo_id": repoID})

	if len(opts.ReleaseIDs) > 0 {
		cond = cond.And(builder.In("release_id", opts.ReleaseIDs))
	}
	return cond
}

// GetDoor43MetadatasByRepoID returns a list of door43 metadatas of repository.
func GetDoor43MetadatasByRepoID(repoID int64, opts FindDoor43MetadatasOptions) ([]*Door43Metadata, error) {
	sess := x.
		Desc("created_unix", "id").
		Where(opts.toConds(repoID))

	if opts.PageSize != 0 {
		sess = opts.setSessionPagination(sess)
	}

	dms := make([]*Door43Metadata, 0, opts.PageSize)
	return dms, sess.Find(&dms)
}

// GetLatestDoor43MetadataByRepoID returns the latest metadata for a repository
func GetLatestDoor43MetadataByRepoID(repoID int64) (*Door43Metadata, error) {
	cond := builder.NewCond().
		And(builder.Eq{"repo_id": repoID}).
		And(builder.Eq{"release_id": 0})

	dm := new(Door43Metadata)
	has, err := x.
		Desc("created_unix", "id").
		Where(cond).
		Get(dm)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReleaseNotExist{0, "latest"}
	}

	return dm, nil
}

// GetDoor43MetadatasByRepoIDAndReleaseIDs returns a list of door43 metadatas of repository according repoID and releaseIDs.
func GetDoor43MetadatasByRepoIDAndReleaseIDs(ctx DBContext, repoID int64, releaseIDs []int64) (dms []*Door43Metadata, err error) {
	err = ctx.e.
		In("release_id", releaseIDs).
		Desc("created_unix").
		Find(&dms, Door43Metadata{RepoID: repoID})
	return dms, err
}

// GetDoor43MetadataCountByRepoID returns the count of metadatas of repository
func GetDoor43MetadataCountByRepoID(repoID int64, opts FindDoor43MetadatasOptions) (int64, error) {
	return x.Where(opts.toConds(repoID)).Count(&Door43Metadata{})
}

type door43MetadataSorter struct {
	dms []*Door43Metadata
}

func (dms *door43MetadataSorter) Len() int {
	return len(dms.dms)
}

func (dms *door43MetadataSorter) Less(i, j int) bool {
	return dms.dms[i].UpdatedUnix > dms.dms[j].UpdatedUnix
}

func (dms *door43MetadataSorter) Swap(i, j int) {
	dms.dms[i], dms.dms[j] = dms.dms[j], dms.dms[i]
}

// SortDoorMetadatas sorts door43 metadatas by number of commits and created time.
func SortDoorMetadatas(dms []*Door43Metadata) {
	sorter := &door43MetadataSorter{dms: dms}
	sort.Sort(sorter)
}

// DeleteDoor43MetadataByID deletes a metadata from database by given ID.
func DeleteDoor43MetadataByID(id int64) error {
	_, err := x.ID(id).Delete(new(Door43Metadata))
	return err
}

// DeleteDoor43MetadataByRelease deletes a metadata from database by given release.
func DeleteDoor43MetadataByRelease(release *Release) error {
	dm, err := GetDoor43Metadata(release.RepoID, release.ID)
	if err != nil {
		return err
	}
	_, err = x.ID(dm.ID).Delete(dm)
	return err
}

// DeleteAllDoor43MetadatasByRepoID deletes all metadatas from database for a repo by given repo ID.
func DeleteAllDoor43MetadatasByRepoID(repoID int64) (int64, error) {
	return x.Delete(Door43Metadata{RepoID: repoID})
}

var rc02Schema []byte

// GetRC02Schema Returns the schema for RC v0.2, retrieving it from file if not already done
func GetRC02Schema() ([]byte, error) {
	if rc02Schema == nil {
		var err error
		rc02Schema, err = GetRepoInitFile("schema", "rc.schema.json")
		if err != nil {
			return nil, err
		}
	}
	return rc02Schema, nil
}

// ProcessDoor43MetadataForRepoRelease handles the metadata for a given repo by release based on if the container is a valid RC or not
func ProcessDoor43MetadataForRepoRelease(repo *Repository, release *Release) error {
	if repo == nil {
		return fmt.Errorf("no repository provided")
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

	entry, err := commit.GetTreeEntryByPath("manifest.yaml")
	if err != nil {
		fmt.Printf("GetTreeEntryByPath Error: %v\n", err)
		return err
	}
	dataRc, err := entry.Blob().DataAsync()
	if err != nil {
		fmt.Printf("DataAsync Error: %v\n", err)
		return err
	}
	defer dataRc.Close()

	content, _ := ioutil.ReadAll(dataRc)
	//fmt.Printf("content: %s", content)

	var manifest map[string]interface{}
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		fmt.Printf("yaml.Unmarshal Error: %v", err)
		return err
	}

	schema, err := GetRC02Schema()
	if err != nil {
		return err
	}
	schemaLoader := gojsonschema.NewBytesLoader(schema)
	documentLoader := gojsonschema.NewGoLoader(manifest)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return err
	}

	var releaseID int64
	if release != nil {
		releaseID = release.ID
	}

	dm, err := GetDoor43Metadata(repo.ID, releaseID)
	if err != nil && !IsErrDoor43MetadataNotExist(err) {
		return err
	}

	if result.Valid() {
		fmt.Printf("The document is valid\n")
		if dm == nil {
			dm = &Door43Metadata{
				RepoID:          repo.ID,
				ReleaseID:       releaseID,
				MetadataVersion: "rc0.2",
				Metadata:        manifest,
			}
			return InsertDoor43Metadata(dm)
		} else if reflect.DeepEqual(dm.Metadata, manifest) {
			dm.Metadata = manifest
			return UpdateDoor43MetadataCols(dm, "metadata")
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
		return DeleteDoor43MetadataByID(dm.ID)
	}
	return nil
}
