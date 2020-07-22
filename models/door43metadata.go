// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"sort"
	"xorm.io/xorm"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"github.com/unknwon/com"

	"xorm.io/builder"
)

// Door43Metadata represents the metadata of repository's release or default branch (ReleaseID = 0).
type Door43Metadata struct {
	ID              int64                   `xorm:"pk autoincr"`
	RepoID          int64                   `xorm:"INDEX UNIQUE(n) NOT NULL"`
	Repo            *Repository             `xorm:"-"`
	ReleaseID       int64                   `xorm:"INDEX UNIQUE(n)"`
	Release         *Release                `xorm:"-"`
	MetadataVersion string                  `xorm:"NOT NULL"`
	Metadata        *map[string]interface{} `xorm:"JSON NOT NULL"`
	CreatedUnix     timeutil.TimeStamp      `xorm:"INDEX created NOT NULL"`
	UpdatedUnix     timeutil.TimeStamp      `xorm:"INDEX updated"`
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
		dm.Release.Door43Metadata = dm
		dm.Release.Repo = dm.Repo
		dm.Release.loadAttributes(e)
	}
	return nil
}

// LoadAttributes load repo and release attributes for a door43 metadata
func (dm *Door43Metadata) LoadAttributes() error {
	return dm.loadAttributes(x)
}

// APIURLV4 the api url for a door43 metadata. door43 metadata must have attributes loaded
func (dm *Door43Metadata) APIURLV4() string {
	ref := dm.Repo.DefaultBranch
	if dm.ReleaseID > 0 {
		ref = dm.Release.TagName
	}
	return fmt.Sprintf("%sapi/catalog/v4/entry/%s/%s",
		setting.AppURL, dm.Repo.FullName(), ref)
}

// HTMLURL the url for a door43 metadata on the web UI. door43 metadata must have attributes loaded
func (dm *Door43Metadata) HTMLURL() string {
	return fmt.Sprintf("%s/metadata/tag/%s", dm.Repo.HTMLURL(), dm.Release.TagName)
}

// APIFormatV4 convert a Door43Metadata to structs.Door43MetadataV4
func (dm *Door43Metadata) APIFormatV4() *structs.Door43MetadataV4 {
	return dm.innerAPIFormatV4(x)
}

func (dm *Door43Metadata) innerAPIFormatV4(e *xorm.Engine) *structs.Door43MetadataV4 {
	dm.loadAttributes(e)
	tag := ""
	stage := StageProd
	released := ""
	releaseURL := ""
	metadataFile := ""
	dm.GetStage()
	if dm.ReleaseID > 0 {
		tag = dm.Release.TagName
		releaseURL = dm.Release.APIURL()
		if dm.Release.IsDraft {
			stage = StageDraft
		} else {
			released = dm.Release.CreatedUnix.FormatDate()
			if dm.Release.IsPrerelease {
				stage = StagePreProd
			} else {
				stage = StageProd
			}
		}
		metadataFile = dm.Repo.HTMLURL() + "/raw/tag/" + dm.Release.TagName + "/manifest.yaml"
	} else {
		tag = dm.Repo.DefaultBranch
		stage = StageLatest
		metadataFile = dm.Repo.HTMLURL() + "/raw/branch/" + dm.Repo.DefaultBranch + "/manifest.yaml"
	}
	return &structs.Door43MetadataV4{
		ID:              dm.ID,
		Self:            dm.APIURLV4(),
		Repo:            dm.Repo.Name,
		Owner:           dm.Repo.OwnerName,
		RepoURL:         dm.Repo.APIURL(),
		ReleaseURL:      releaseURL,
		Language:        (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string),
		Subject:         (*dm.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string),
		Title:           (*dm.Metadata)["dublin_core"].(map[string]interface{})["title"].(string),
		Books:           dm.GetBooks(),
		Tag:             tag,
		Stage:           stage.String(),
		Released:        released,
		MetadataVersion: dm.MetadataVersion,
		MetadataURL:     dm.APIURLV4() + "/metadata",
		MetadataFile:    metadataFile,
		Ingredients:     (*dm.Metadata)["projects"].([]interface{}),
	}
}

// GetBooks get the books of the resource
func (dm *Door43Metadata) GetBooks() []string {
	var books []string
	if len((*dm.Metadata)["projects"].([]interface{})) > 0 {
		for _, prod := range (*dm.Metadata)["projects"].([]interface{}) {
			books = append(books, prod.(map[string]interface{})["identifier"].(string))
		}
	}
	return books
}

// IsDoor43MetadataExist returns true if door43 metadata with given release ID already exists.
func IsDoor43MetadataExist(repoID, releaseID int64) (bool, error) {
	return x.Get(&Door43Metadata{RepoID: repoID, ReleaseID: releaseID})
}

// InsertDoor43Metadata inserts a door43 metadata
func InsertDoor43Metadata(dm *Door43Metadata) error {
	id, err := x.Insert(dm)
	if id > 0 && dm.ReleaseID > 0 {
		dm.LoadAttributes()
		if err := CreateRepositoryNotice("Door43 Metadata created for repo: %s, tag: %s", dm.Repo.Name, dm.Release.TagName); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
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
	id, err := e.ID(dm.ID).Cols(cols...).Update(dm)
	if id > 0 && dm.ReleaseID > 0 {
		dm.LoadAttributes()
		if err := CreateRepositoryNotice("Door43 Metadata updated for repo: %s, tag: %s", dm.Repo.Name, dm.Release.TagName); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return err
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
		return nil, ErrDoor43MetadataNotExist{0, repoID, releaseID}
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
		return nil, ErrDoor43MetadataNotExist{id, 0, 0}
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

	if opts.PageSize > 0 {
		sess = opts.setSessionPagination(sess)
	}

	dms := make([]*Door43Metadata, 0, opts.PageSize)
	return dms, sess.Find(&dms)
}

// GetLatestCatalogMetadataByRepoID returns the latest door43 metadata in the catalog by repoID, if CanBePrerelease, a prerelease entry can match
func GetLatestCatalogMetadataByRepoID(repoID int64, CanBePrerelease bool) (*Door43Metadata, error) {
	return getLatestCatalogMetadataByRepoID(x, repoID, CanBePrerelease)
}

func getLatestCatalogMetadataByRepoID(e Engine, repoID int64, CanBePrerelease bool) (*Door43Metadata, error) {
	cond := builder.NewCond().
		And(builder.Eq{"`door43_metadata`.repo_id": repoID}).
		And(builder.Eq{"`release`.is_tag": 0}).
		And(builder.Eq{"`release`.is_draft": 0})

	if !CanBePrerelease {
		cond = cond.And(builder.Eq{"`release`.is_prerelease": 0})
	}

	dm := new(Door43Metadata)
	has, err := e.
		Join("INNER", "release", "`release`.id = `door43_metadata`.release_id").
		Where(cond).
		Desc("`release`.created_unix", "`release`.id").
		Get(dm)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDoor43MetadataNotExist{0, repoID, 0}
	}
	dm.loadAttributes(e)
	return dm, nil
}

// GetDoor43MetadatasByRepoIDAndReleaseIDs returns a list of door43 metadatas of repository according repoID and releaseIDs.
func GetDoor43MetadatasByRepoIDAndReleaseIDs(repoID int64, releaseIDs []int64) (dms []*Door43Metadata, err error) {
	err = x.In("release_id", releaseIDs).
		Desc("created_unix").
		Find(&dms, Door43Metadata{RepoID: repoID})
	return dms, err
}

// GetDoor43MetadataCountByRepoID returns the count of metadatas of repository
func GetDoor43MetadataCountByRepoID(repoID int64, opts FindDoor43MetadatasOptions) (int64, error) {
	return x.Where(opts.toConds(repoID)).Count(&Door43Metadata{})
}

// GetDoor43MetadataReleaseCountByRepoID returns the count of metadatas of repository that are releases
func GetDoor43MetadataReleaseCountByRepoID(repoID int64, includePreproduction bool) (int64, error) {
	releaseCondition := "`release`.id = `door43_metadata`.release_id"
	if includePreproduction {
		releaseCondition += "`release`.is_prerelease = 0"
	}
	return x.Join("INNER", "release", releaseCondition).
		Where(builder.And(builder.Eq{"`door43_metadata`.repo_id": repoID})).
		Count(&Door43Metadata{})
}

// GetDoor43MetadataByRepoIDAndReleaseID returns the metadata of a given release ID (0 = default branch).
func GetDoor43MetadataByRepoIDAndReleaseID(repoID, releaseID int64) (*Door43Metadata, error) {
	return getDoor43MetadataByRepoIDAndReleaseID(x, repoID, releaseID)
}

func getDoor43MetadataByRepoIDAndReleaseID(e Engine, repoID, releaseID int64) (*Door43Metadata, error) {
	dm := &Door43Metadata{RepoID: repoID, ReleaseID: releaseID}
	has, err := e.Get(dm)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDoor43MetadataNotExist{0, repoID, releaseID}
	}
	return dm, err
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
	dm, err := GetDoor43MetadataByID(id)
	if err != nil {
		return err
	}
	dm.LoadAttributes()
	return DeleteDoor43Metadata(dm)
}

// DeleteDoor43Metadata deletes a metadata from database by given ID.
func DeleteDoor43Metadata(dm *Door43Metadata) error {
	id, err := x.Delete(dm)
	if id > 0 && dm.ReleaseID > 0 {
		dm.LoadAttributes()
		if err := CreateRepositoryNotice("Door43 Metadata deleted for repo: %s, tag: %s", dm.Repo.Name, dm.Release.TagName); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return err
}

// DeleteDoor43MetadataByRelease deletes a metadata from database by given release.
func DeleteDoor43MetadataByRelease(release *Release) error {
	dm, err := GetDoor43MetadataByRepoIDAndReleaseID(release.RepoID, release.ID)
	if err != nil {
		if !IsErrDoor43MetadataNotExist(err) {
			return err
		}
		return nil
	}
	_, err = x.ID(dm.ID).Delete(dm)
	return err
}

// DeleteAllDoor43MetadatasByRepoID deletes all metadatas from database for a repo by given repo ID.
func DeleteAllDoor43MetadatasByRepoID(repoID int64) (int64, error) {
	return x.Delete(Door43Metadata{RepoID: repoID})
}

// GetRepoIDsNeedingDoor43Metadata gets the IDs of all the repos that may have releases or default branch not in the door43_metadata table.
func GetRepoIDsNeedingDoor43Metadata() ([]int64, error) {
	sess := x.NewSession()
	defer sess.Close()

	records, err := sess.Query("SELECT r.id FROM `repository` r " +
		"JOIN `release` rel ON rel.repo_id = r.id " +
		"LEFT JOIN `door43_metadata` dm ON r.id = dm.repo_id " +
		"AND rel.id = dm.release_id " +
		"WHERE dm.id IS NULL AND rel.is_tag = 0 " +
		"UNION " +
		"SELECT r2.id FROM `repository` r2 " +
		"LEFT JOIN `door43_metadata` dm2 ON r2.id = dm2.repo_id " +
		"AND dm2.release_id = 0 " +
		"WHERE dm2.id IS NULL " +
		"ORDER BY id ASC")
	if err != nil {
		return nil, err
	}

	repoIDs := make([]int64, len(records))

	for idx, record := range records {
		repoIDs[idx] = com.StrTo(record["id"]).MustInt64()
	}

	return repoIDs, nil
}

// GetReleaseIDsNeedingDoor43Metadata gets the releases needing door43 metadata
func (r *Repository) GetReleaseIDsNeedingDoor43Metadata() ([]int64, error) {
	sess := x.NewSession()
	defer sess.Close()

	records, err := sess.Query("SELECT rel.id as id FROM `repository` r "+
		"JOIN `release` rel ON rel.repo_id = r.id "+
		"LEFT JOIN `door43_metadata` dm ON r.id = dm.repo_id "+
		"AND rel.id = dm.release_id "+
		"WHERE dm.id IS NULL AND rel.is_tag = 0 AND r.id=? "+
		"UNION "+
		"SELECT 0 as id FROM `repository` r2 "+
		"LEFT JOIN `door43_metadata` dm2 ON r2.id = dm2.repo_id "+
		"AND dm2.release_id = 0 "+
		"WHERE dm2.id IS NULL AND r2.id=? "+
		"ORDER BY id ASC", r.ID, r.ID)
	log.Trace(sess.LastSQL())
	if err != nil {
		return nil, err
	}

	relIDs := make([]int64, len(records))

	for idx, record := range records {
		relIDs[idx] = com.StrTo(record["id"]).MustInt64()
	}

	return relIDs, nil
}

// GetLatestProdCatalogMetadata gets the latest Door43 Metadata that is in the prod catalog.
func (r *Repository) GetLatestProdCatalogMetadata() (*Door43Metadata, error) {
	return r.getLatestProdCatalogMetadata(x)
}

func (r *Repository) getLatestProdCatalogMetadata(e Engine) (*Door43Metadata, error) {
	dm, err := GetLatestCatalogMetadataByRepoID(r.ID, false)
	if err != nil && !IsErrDoor43MetadataNotExist(err) {
		return nil, err
	}
	return dm, nil
}

// GetLatestPreProdCatalogMetadata gets the latest Door43 Metadata that is in the pre-prod catalog.
func (r *Repository) GetLatestPreProdCatalogMetadata() (*Door43Metadata, error) {
	return r.getLatestPreProdCatalogMetadata(x)
}

func (r *Repository) getLatestPreProdCatalogMetadata(e Engine) (*Door43Metadata, error) {
	dm, err := getLatestCatalogMetadataByRepoID(e, r.ID, true)
	if err != nil && !IsErrDoor43MetadataNotExist(err) {
		return nil, err
	}
	if dm != nil && !dm.Release.IsPrerelease {
		dm = nil
	}
	return dm, nil
}

// GetCatalogReleaseCount gets the number of all valid releases for a repo
func (r *Repository) GetCatalogReleaseCount() (int64, error) {
	return GetDoor43MetadataReleaseCountByRepoID(r.ID, true)
}

// GetProdCatalogReleaseCount gets the number of valid prod releases for a repo
func (r *Repository) GetProdCatalogReleaseCount() (int64, error) {
	return GetDoor43MetadataReleaseCountByRepoID(r.ID, false)
}

// GetDefaultBranchMetadata gets the default branch's Door43 Metadata.
func (r *Repository) GetDefaultBranchMetadata() (*Door43Metadata, error) {
	dm, err := GetDoor43MetadataByRepoIDAndReleaseID(r.ID, 0)
	if err != nil && !IsErrDoor43MetadataNotExist(err) {
		return nil, err
	}
	return dm, nil
}

/*** Error Structs & Functions ***/

// ErrDoor43MetadataAlreadyExist represents a "Door43MetadataAlreadyExist" kind of error.
type ErrDoor43MetadataAlreadyExist struct {
	ReleaseID int64
}

// IsErrDoor43MetadataAlreadyExist checks if an error is a ErrDoor43MetadataAlreadyExist.
func IsErrDoor43MetadataAlreadyExist(err error) bool {
	_, ok := err.(ErrDoor43MetadataAlreadyExist)
	return ok
}

func (err ErrDoor43MetadataAlreadyExist) Error() string {
	return fmt.Sprintf("Metadata for release already exists [release: %d]", err.ReleaseID)
}

// ErrDoor43MetadataNotExist represents a "Door43MetadataNotExist" kind of error.
type ErrDoor43MetadataNotExist struct {
	ID        int64
	RepoID    int64
	ReleaseID int64
}

// IsErrDoor43MetadataNotExist checks if an error is a ErrDoor43MetadataNotExist.
func IsErrDoor43MetadataNotExist(err error) bool {
	_, ok := err.(ErrDoor43MetadataNotExist)
	return ok
}

func (err ErrDoor43MetadataNotExist) Error() string {
	return fmt.Sprintf("metadata release id does not exist [id: %d, release_id: %s]", err.ID, err.ReleaseID)
}

// ErrInvalidRelease represents a "InvalidRelease" kind of error.
type ErrInvalidRelease struct {
	ReleaseID int64
}

// IsErrInvalidRelease checks if an error is a ErrInvalidRelease.
func IsErrInvalidRelease(err error) bool {
	_, ok := err.(ErrInvalidRelease)
	return ok
}

func (err ErrInvalidRelease) Error() string {
	return fmt.Sprintf("metadata release id is not valid [release_id: %d]", err.ReleaseID)
}

/*** END Error Structs & Functions ***/

/*** Stage ***/

// Stage type for choosing which level of stage to return in the Catalog results
type Stage int

// Stage values
const (
	StageProd    Stage = 1
	StagePreProd Stage = 2
	StageDraft   Stage = 3
	StageLatest  Stage = 4
)

// StageMap map from string to Stage (int)
var StageMap = map[string]Stage{
	"prod":    StageProd,
	"preprod": StagePreProd,
	"draft":   StageDraft,
	"latest":  StageLatest,
}

// StageToStringMap map from stage (int) to string
var StageToStringMap = map[Stage]string{
	StageProd:    "prod",
	StagePreProd: "preprod",
	StageDraft:   "draft",
	StageLatest:  "latest",
}

// String returns string repensation of a Stage (int)
func (s *Stage) String() string {
	return StageToStringMap[*s]
}

// GetStage gets the Stage (int) for a given Door43Metadata
func (dm *Door43Metadata) GetStage() Stage {
	dm.LoadAttributes()
	if dm.ReleaseID > 0 {
		if dm.Release.IsDraft {
			return StageDraft
		} else if dm.Release.IsPrerelease {
			return StagePreProd
		} else {
			return StageProd
		}
	}
	return StageLatest
}

/*** END Stage ***/
