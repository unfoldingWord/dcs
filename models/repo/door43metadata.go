// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

/*** INIT DB ***/

// InitDoor43Metadata does some db management
func InitDoor43Metadata() error {
	switch setting.Database.Type {
	case "mysql":
		_, err := db.GetEngine(db.DefaultContext).Exec("ALTER TABLE `door43_metadata` MODIFY `metadata` JSON")
		if err != nil {
			return fmt.Errorf("Error changing door43_metadata metadata column type: %v", err)
		}
	}
	return nil
}

/*** END INIT DB ***/

/*** START Door43Metadata struct and getters ***/

// Door43Metadata represents the metadata of repository's release or default branch (ReleaseID = 0).
type Door43Metadata struct {
	ID              int64                   `xorm:"pk autoincr"`
	RepoID          int64                   `xorm:"INDEX UNIQUE(n) NOT NULL"`
	Repo            *Repository             `xorm:"-"`
	ReleaseID       int64                   `xorm:"INDEX UNIQUE(n)"`
	Release         *Release                `xorm:"-"`
	MetadataVersion string                  `xorm:"NOT NULL"`
	Metadata        *map[string]interface{} `xorm:"JSON NOT NULL"`
	Stage           door43metadata.Stage    `xorm:"NOT NULL"`
	BranchOrTag     string                  `xorm:"NOT NULL"`
	ReleaseDateUnix timeutil.TimeStamp      `xorm:"NOT NULL"`
	CreatedUnix     timeutil.TimeStamp      `xorm:"INDEX created NOT NULL"`
	UpdatedUnix     timeutil.TimeStamp      `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(Door43Metadata))
}

// GetRepo gets the repo associated with the door43 metadata entry
func (dm *Door43Metadata) GetRepo() error {
	return dm.getRepo(db.DefaultContext)
}

func (dm *Door43Metadata) getRepo(ctx context.Context) error {
	if dm.Repo == nil {
		repo, err := GetRepositoryByID(ctx, dm.RepoID)
		if err != nil {
			return err
		}
		dm.Repo = repo
		if err := dm.Repo.GetOwner(ctx); err != nil {
			return err
		}
	}
	return nil
}

// GetRelease gets the associated release of a door43 metadata entry
func (dm *Door43Metadata) GetRelease() error {
	return dm.getRelease(db.DefaultContext)
}

func (dm *Door43Metadata) getRelease(ctx context.Context) error {
	if dm.ReleaseID > 0 && dm.Release == nil {
		rel, err := GetReleaseByID(ctx, dm.ReleaseID)
		if err != nil {
			return err
		}
		dm.Release = rel
		dm.Release.Door43Metadata = dm
		if err := dm.getRepo(ctx); err != nil {
			return err
		}
		dm.Release.Repo = dm.Repo
		if err := dm.Release.LoadAttributes(ctx); err != nil {
			log.Warn("loadAttributes Error: %v\n", err)
			return err
		}
	}
	return nil
}

func (dm *Door43Metadata) loadAttributes(ctx context.Context) error {
	if err := dm.getRepo(ctx); err != nil {
		return err
	}
	if dm.Release == nil && dm.ReleaseID > 0 {
		if err := dm.getRelease(ctx); err != nil {
			log.Error("getRelease: %v", err)
			return nil
		}
	}
	return nil
}

// LoadAttributes load repo and release attributes for a door43 metadata
func (dm *Door43Metadata) LoadAttributes() error {
	return dm.loadAttributes(db.DefaultContext)
}

// GetBranchOrTagType gets the type of the DM entry, "branch" or "tag"
func (dm *Door43Metadata) GetBranchOrTagType() string {
	if dm.Stage < door43metadata.StageDraft {
		return "tag"
	}
	return "branch"
}

// APIURL the api url for a door43 metadata. door43 metadata must have attributes loaded
func (dm *Door43Metadata) APIURL() string {
	ref := dm.Repo.DefaultBranch
	if dm.ReleaseID > 0 {
		ref = dm.Release.TagName
	}
	return fmt.Sprintf("%sapi/v1/catalog/entry/%s/%s/", setting.AppURL, dm.Repo.FullName(), ref)
}

// HTMLURL the url for a door43 metadata on the web UI. door43 metadata must have attributes loaded
func (dm *Door43Metadata) HTMLURL() string {
	return fmt.Sprintf("%s/metadata/tag/%s", dm.Repo.HTMLURL(), dm.Release.TagName)
}

// GetTarballURL get the tarball URL of the tag or branch
func (dm *Door43Metadata) GetTarballURL() string {
	return fmt.Sprintf("%s/archive/%s.tar.gz", dm.Repo.HTMLURL(), dm.BranchOrTag)
}

// GetZipballURL get the zipball URL of the tag or branch
func (dm *Door43Metadata) GetZipballURL() string {
	return fmt.Sprintf("%s/archive/%s.zip", dm.Repo.HTMLURL(), dm.BranchOrTag)
}

// GetReleaseURL get the URL the release API
func (dm *Door43Metadata) GetReleaseURL() string {
	if dm.ReleaseID > 0 {
		if dm.Release != nil {
			return dm.Release.APIURL()
		}
		if err := dm.GetRepo(); err == nil {
			return fmt.Sprintf("%sapi/v1/repos/%s/releases/%d", setting.AppURL, dm.Repo.FullName(), dm.ReleaseID)
		}
	}
	return ""
}

// GetMetadataURL gets the url to the raw manifest.yaml file
func (dm *Door43Metadata) GetMetadataURL() string {
	return fmt.Sprintf("%s/raw/%s/%s/manifest.yaml", dm.Repo.HTMLURL(), dm.GetBranchOrTagType(), dm.BranchOrTag)
}

// GetMetadataJSONURL gets the json representation of the contents of the manifest.yaml file
func (dm *Door43Metadata) GetMetadataJSONURL() string {
	return fmt.Sprintf("%smetadata/", dm.APIURL())
}

// GetMetadataAPIContentsURL gets the metadata API contents URL of the manifest.yaml file
func (dm *Door43Metadata) GetMetadataAPIContentsURL() string {
	return fmt.Sprintf("%s/contents/manifest.yaml?ref=%s", dm.Repo.APIURL(), dm.BranchOrTag)
}

// GetGitTreesURL gets the git trees URL for a repo and branch or tag for all files
func (dm *Door43Metadata) GetGitTreesURL() string {
	return fmt.Sprintf("%s/git/trees/%s?recursive=1&per_page=99999", dm.Repo.APIURL(), dm.BranchOrTag)
}

// GetContentsURL gets the contents URL for a repo and branch or tag for all files
func (dm *Door43Metadata) GetContentsURL() string {
	return fmt.Sprintf("%s/contents?ref=%s", dm.Repo.APIURL(), dm.BranchOrTag)
}

// GetBooks get the books of the manifest
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
	return db.GetEngine(db.DefaultContext).Get(&Door43Metadata{RepoID: repoID, ReleaseID: releaseID})
}

// InsertDoor43Metadata inserts a door43 metadata
func InsertDoor43Metadata(dm *Door43Metadata) error {
	if id, err := db.GetEngine(db.DefaultContext).Insert(dm); err != nil {
		return err
	} else if id > 0 && dm.ReleaseID > 0 {
		if err := dm.LoadAttributes(); err != nil {
			return err
		}
		if err := system.CreateRepositoryNotice("Door43 Metadata created for repo: %s, tag: %s", dm.Repo.Name, dm.Release.TagName); err != nil {
			return err
		}
	}
	return nil
}

// InsertDoor43MetadatasContext inserts door43 metadatas
func InsertDoor43MetadatasContext(ctx db.Context, dms []*Door43Metadata) error {
	_, err := ctx.Engine().Insert(dms)
	return err
}

// UpdateDoor43MetadataCols update door43 metadata according special columns
func UpdateDoor43MetadataCols(dm *Door43Metadata, cols ...string) error {
	return updateDoor43MetadataCols(db.GetEngine(db.DefaultContext), dm, cols...)
}

func updateDoor43MetadataCols(e db.Engine, dm *Door43Metadata, cols ...string) error {
	id, err := e.ID(dm.ID).Cols(cols...).Update(dm)
	if id > 0 && dm.ReleaseID > 0 {
		err := dm.LoadAttributes()
		if err != nil {
			return err
		}
		if err := system.CreateRepositoryNotice("Door43 Metadata updated for repo: %s, tag: %s", dm.Repo.Name, dm.Release.TagName); err != nil {
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
	_, err = db.GetEngine(db.DefaultContext).Get(dm)
	return dm, err
}

// GetDoor43MetadataByID returns door43 metadata with given ID.
func GetDoor43MetadataByID(id int64) (*Door43Metadata, error) {
	rel := new(Door43Metadata)
	has, err := db.GetEngine(db.DefaultContext).
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
	db.ListOptions
	ReleaseIDs []int64
}

func (opts *FindDoor43MetadatasOptions) toConds(repoID int64) builder.Cond {
	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"repo_id": repoID})

	if len(opts.ReleaseIDs) > 0 {
		cond = cond.And(builder.In("release_id", opts.ReleaseIDs))
	}
	return cond
}

// GetDoor43MetadatasByRepoID returns a list of door43 metadatas of repository.
func GetDoor43MetadatasByRepoID(repoID int64, opts FindDoor43MetadatasOptions) ([]*Door43Metadata, error) {
	sess := db.GetEngine(db.DefaultContext).
		Desc("created_unix", "id").
		Where(opts.toConds(repoID))

	if opts.PageSize > 0 {
		sess = db.SetSessionPagination(sess, &opts)
	}

	dms := make([]*Door43Metadata, 0, opts.PageSize)
	return dms, sess.Find(&dms)
}

// GetLatestCatalogMetadataByRepoID returns the latest door43 metadata in the catalog by repoID, if canBePrerelease, a prerelease entry can match
func GetLatestCatalogMetadataByRepoID(repoID int64, canBePrerelease bool) (*Door43Metadata, error) {
	return getLatestCatalogMetadataByRepoID(db.DefaultContext, repoID, canBePrerelease)
}

func getLatestCatalogMetadataByRepoID(ctx context.Context, repoID int64, canBePrerelease bool) (*Door43Metadata, error) {
	cond := builder.NewCond().
		And(builder.Eq{"`door43_metadata`.repo_id": repoID}).
		And(builder.Eq{"`release`.is_tag": 0}).
		And(builder.Eq{"`release`.is_draft": 0})

	if !canBePrerelease {
		cond = cond.And(builder.Eq{"`release`.is_prerelease": 0})
	}

	dm := new(Door43Metadata)
	has, err := db.GetEngine(ctx).
		Join("INNER", "release", "`release`.id = `door43_metadata`.release_id").
		Where(cond).
		Desc("`release`.created_unix", "`release`.id").
		Get(dm)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDoor43MetadataNotExist{0, repoID, 0}
	}

	return dm, dm.loadAttributes(ctx)
}

// GetDoor43MetadatasByRepoIDAndReleaseIDs returns a list of door43 metadatas of repository according repoID and releaseIDs.
func GetDoor43MetadatasByRepoIDAndReleaseIDs(repoID int64, releaseIDs []int64) (dms []*Door43Metadata, err error) {
	err = db.GetEngine(db.DefaultContext).In("release_id", releaseIDs).
		Desc("created_unix").
		Find(&dms, Door43Metadata{RepoID: repoID})
	return dms, err
}

// GetDoor43MetadataCountByRepoID returns the count of metadatas of repository
func GetDoor43MetadataCountByRepoID(repoID int64, opts FindDoor43MetadatasOptions) (int64, error) {
	return db.GetEngine(db.DefaultContext).Where(opts.toConds(repoID)).Count(&Door43Metadata{})
}

// GetReleaseCount returns the count of releases of repository of the Door43Metadata's stage
func (dm *Door43Metadata) GetReleaseCount() (int64, error) {
	stageCond := door43metadata.GetStageCond(dm.Stage)
	return db.GetEngine(db.DefaultContext).Join("LEFT", "release", "`release`.id = `door43_metadata`.release_id").
		Where(builder.And(builder.Eq{"`door43_metadata`.repo_id": dm.RepoID}, stageCond)).
		Count(&Door43Metadata{})
}

// GetDoor43MetadataByRepoIDAndReleaseID returns the metadata of a given release ID (0 = default branch).
func GetDoor43MetadataByRepoIDAndReleaseID(repoID, releaseID int64) (*Door43Metadata, error) {
	return getDoor43MetadataByRepoIDAndReleaseID(db.GetEngine(db.DefaultContext), repoID, releaseID)
}

func getDoor43MetadataByRepoIDAndReleaseID(e db.Engine, repoID, releaseID int64) (*Door43Metadata, error) {
	var dms []*Door43Metadata
	err := e.Find(&dms, &Door43Metadata{RepoID: repoID, ReleaseID: releaseID})
	if err != nil {
		return nil, err
	}
	if len(dms) > 0 {
		for _, dm := range dms {
			if dm.ReleaseID == releaseID {
				return dm, nil
			}
		}
	}
	return nil, ErrDoor43MetadataNotExist{0, repoID, releaseID}
}

// GetDoor43MetadataByRepoIDAndStage returns the metadata of a given repo ID and stage.
func GetDoor43MetadataByRepoIDAndStage(repoID int64, stage door43metadata.Stage) (*Door43Metadata, error) {
	return getDoor43MetadataByRepoIDAndStage(db.GetEngine(db.DefaultContext), repoID, stage)
}

func getDoor43MetadataByRepoIDAndStage(e db.Engine, repoID int64, stage door43metadata.Stage) (*Door43Metadata, error) {
	cond := builder.NewCond().
		And(builder.Eq{"repo_id": repoID}).
		And(builder.Eq{"stage": stage})
	e = e.Where(cond).Desc("release_date_unix").Desc("branch_or_tag")

	dm := &Door43Metadata{}
	found, err := e.Get(dm)
	if err != nil || !found {
		return nil, err
	}
	return dm, err
}

// GetLatestProdCatalogMetadata gets the latest Door43 Metadata that is in the prod catalog.
func GetLatestProdCatalogMetadata(repoID int64) (*Door43Metadata, error) {
	return getLatestProdCatalogMetadata(repoID, db.GetEngine(db.DefaultContext))
}

func getLatestProdCatalogMetadata(repoID int64, e db.Engine) (*Door43Metadata, error) {
	dm, err := GetLatestCatalogMetadataByRepoID(repoID, false)
	if err != nil && !IsErrDoor43MetadataNotExist(err) {
		return nil, err
	}
	return dm, nil
}

// GetLatestPreProdCatalogMetadata gets the latest Door43 Metadata that is in the pre-prod catalog.
func GetLatestPreProdCatalogMetadata(repoID int64) (*Door43Metadata, error) {
	return getLatestPreProdCatalogMetadata(db.DefaultContext, repoID, db.GetEngine(db.DefaultContext))
}

func getLatestPreProdCatalogMetadata(ctx context.Context, repoID int64, e db.Engine) (*Door43Metadata, error) {
	dm, err := getLatestCatalogMetadataByRepoID(ctx, repoID, true)
	if err != nil && !IsErrDoor43MetadataNotExist(err) {
		return nil, err
	}
	if dm != nil && !dm.Release.IsPrerelease {
		dm = nil
	}
	return dm, nil
}

// GetDefaultBranchMetadata gets the default branch's Door43 Metadata.
func GetDefaultBranchMetadata(repoID int64) (*Door43Metadata, error) {
	dm, err := GetDoor43MetadataByRepoIDAndReleaseID(repoID, 0)
	if err != nil && !IsErrDoor43MetadataNotExist(err) {
		return nil, err
	}
	return dm, nil
}

// GetDoor43MetadataMapValues gets the values of a Door43Metadata map
func GetDoor43MetadataMapValues(m map[int64]*Door43Metadata) []*Door43Metadata {
	values := make([]*Door43Metadata, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

/*** END Door43Metadata struct and getters ***/

/*** START Door43MetadataList ***/

// Door43MetadataList contains a list of repositories
type Door43MetadataList []*Door43Metadata

func (dms Door43MetadataList) Len() int {
	return len(dms)
}

func (dms Door43MetadataList) Less(i, j int) bool {
	return dms[i].Repo.FullName() < dms[j].Repo.FullName()
}

func (dms Door43MetadataList) Swap(i, j int) {
	dms[i], dms[j] = dms[j], dms[i]
}

// Door43MetadataListOfMap make list from values of map
func Door43MetadataListOfMap(dmMap map[int64]*Door43Metadata) Door43MetadataList {
	return Door43MetadataList(GetDoor43MetadataMapValues(dmMap))
}

// LoadAttributes loads the attributes for the given Door43MetadataList
func (dms Door43MetadataList) LoadAttributes() error {
	return dms.loadAttributes(db.DefaultContext)
}

func (dms Door43MetadataList) loadAttributes(ctx context.Context) error {
	if len(dms) == 0 {
		return nil
	}
	var lastErr error
	for _, dm := range dms {
		if err := dm.loadAttributes(ctx); err != nil && lastErr == nil {
			lastErr = err
		}
	}
	return lastErr
}

/*** END Door43MEtadataList ***/

/*** Door43MetadataSorter ***/
type Door43MetadataSorter struct {
	dms []*Door43Metadata
}

func (dms *Door43MetadataSorter) Len() int {
	return len(dms.dms)
}

func (dms *Door43MetadataSorter) Less(i, j int) bool {
	return dms.dms[i].UpdatedUnix > dms.dms[j].UpdatedUnix
}

func (dms *Door43MetadataSorter) Swap(i, j int) {
	dms.dms[i], dms.dms[j] = dms.dms[j], dms.dms[i]
}

// SortDoorMetadatas sorts door43 metadatas by number of commits and created time.
func SortDoorMetadatas(dms []*Door43Metadata) {
	sorter := &Door43MetadataSorter{dms: dms}
	sort.Sort(sorter)
}

// DeleteDoor43MetadataByID deletes a metadata from database by given ID.
func DeleteDoor43MetadataByID(id int64) error {
	if dm, err := GetDoor43MetadataByID(id); err != nil {
		return err
	} else if err := dm.LoadAttributes(); err != nil {
		return err
	} else {
		return DeleteDoor43Metadata(dm)
	}
}

// DeleteDoor43Metadata deletes a metadata from database by given ID.
func DeleteDoor43Metadata(dm *Door43Metadata) error {
	id, err := db.GetEngine(db.DefaultContext).Delete(dm)
	if id > 0 && dm.ReleaseID > 0 {
		if err := dm.LoadAttributes(); err != nil {
			return err
		} else if err := system.CreateRepositoryNotice("Door43 Metadata deleted for repo: %s, tag: %s", dm.Repo.Name, dm.Release.TagName); err != nil {
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
	_, err = db.GetEngine(db.DefaultContext).ID(dm.ID).Delete(dm)
	return err
}

// DeleteAllDoor43MetadatasByRepoID deletes all metadatas from database for a repo by given repo ID.
func DeleteAllDoor43MetadatasByRepoID(repoID int64) (int64, error) {
	return db.GetEngine(db.DefaultContext).Delete(Door43Metadata{RepoID: repoID})
}

// GetReposForMetadata gets the IDs of all the repos to process for metadata
func GetReposForMetadata() ([]int64, error) {
	// records, err := db.GetEngine(db.DefaultContext).Query("SELECT r.id FROM `repository` r " +
	//	"JOIN `release` rel ON rel.repo_id = r.id " +
	//	"LEFT JOIN `door43_metadata` dm ON r.id = dm.repo_id " +
	//	"AND rel.id = dm.release_id " +
	//	"WHERE dm.id IS NULL AND rel.is_tag = 0 " +
	//	"UNION " +
	//	"SELECT r2.id FROM `repository` r2 " +
	//	"LEFT JOIN `door43_metadata` dm2 ON r2.id = dm2.repo_id " +
	//	"AND dm2.release_id = 0 " +
	//	"WHERE dm2.id IS NULL " +
	//	"ORDER BY id ASC")
	// records, err := sess.Query("SELECT r.id FROM `repository` r " +
	//	"ORDER BY id ASC")
	// records, err := sess.Query("SELECT r.id FROM `repository` r WHERE r.is_private = 0 AND r.is_archived = 0 ORDER BY id ASC")
	records, err := db.GetEngine(db.DefaultContext).Query("SELECT id FROM `repository` ORDER BY id ASC")
	if err != nil {
		return nil, err
	}

	repoIDs := make([]int64, len(records))

	for idx, record := range records {
		v, _ := strconv.ParseInt(string(record["id"]), 10, 64)
		repoIDs[idx] = v
	}

	return repoIDs, nil
}

// GetRepoReleaseIDsForMetadata gets the releases ids for a repo
func GetRepoReleaseIDsForMetadata(repoID int64) ([]int64, error) {
	// records, err := db.GetEngine(db.DefaultContext).Query("SELECT rel.id as id FROM `repository` r "+
	//	"INNER JOIN `release` rel ON rel.repo_id = r.id "+
	//	"LEFT JOIN `door43_metadata` dm ON r.id = dm.repo_id "+
	//	"AND rel.id = dm.release_id "+
	//	"WHERE dm.id IS NULL AND rel.is_tag = 0 AND r.id=? "+
	//	"UNION "+
	//	"SELECT 0 as id FROM `repository` r2 "+
	//	"LEFT JOIN `door43_metadata` dm2 ON r2.id = dm2.repo_id "+
	//	"AND dm2.release_id = 0 "+
	//	"WHERE dm2.id IS NULL AND r2.id=? "+
	//	"ORDER BY id ASC", r.ID, r.ID)
	records, err := db.GetEngine(db.DefaultContext).Query("SELECT rel.id as id FROM `repository` r "+
		"INNER JOIN `release` rel ON rel.repo_id = r.id "+
		"WHERE rel.is_tag = 0 AND r.id=? "+
		"UNION "+
		"SELECT 0 as id FROM `repository` r2 "+
		"WHERE r2.id=? "+
		"ORDER BY id ASC", repoID, repoID)
	if err != nil {
		return nil, err
	}

	relIDs := make([]int64, len(records))

	for idx, record := range records {
		v, _ := strconv.ParseInt(string(record["id"]), 10, 64)
		relIDs[idx] = v
	}

	return relIDs, nil
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
	return fmt.Sprintf("metadata release id does not exist [id: %d, release_id: %d]", err.ID, err.ReleaseID)
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
