// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"sort"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
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
	ID                int64                   `xorm:"pk autoincr"`
	RepoID            int64                   `xorm:"INDEX UNIQUE(repo_ref) NOT NULL"`
	Repo              *Repository             `xorm:"-"`
	ReleaseID         int64                   `xorm:"NOT NULL"`
	Release           *Release                `xorm:"-"`
	Ref               string                  `xorm:"INDEX UNIQUE(repo_ref) NOT NULL"`
	RefType           string                  `xorm:"NOT NULL"`
	CommitSHA         string                  `xorm:"NOT NULL VARCHAR(40)"`
	Stage             door43metadata.Stage    `xorm:"NOT NULL"`
	MetadataType      string                  `xorm:"NOT NULL"`
	MetadataVersion   string                  `xorm:"NOT NULL"`
	Resource          string                  `xorm:"NOT NULL"`
	Subject           string                  `xorm:"NOT NULL"`
	Title             string                  `xorm:"NOT NULL"`
	Language          string                  `xorm:"NOT NULL"`
	LanguageTitle     string                  `xorm:"NOT NULL"`
	LanguageDirection string                  `xorm:"NOT NULL"`
	LanguageIsGL      bool                    `xorm:"NOT NULL"`
	ContentFormat     string                  `xorm:"NOT NULL"`
	CheckingLevel     int                     `xorm:"NOT NULL"`
	Ingredients       []*structs.Ingredient   `xorm:"JSON"`
	Metadata          *map[string]interface{} `xorm:"JSON"`
	ReleaseDateUnix   timeutil.TimeStamp      `xorm:"NOT NULL"`
	CreatedUnix       timeutil.TimeStamp      `xorm:"INDEX created NOT NULL"`
	UpdatedUnix       timeutil.TimeStamp      `xorm:"INDEX updated"`
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
		if err := dm.Repo.LoadOwner(ctx); err != nil {
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
		// if err := dm.Release.LoadAttributes(); err != nil {
		// 	log.Warn("loadAttributes Error: %v\n", err)
		// 	return err
		// }
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

// APIURL the api url for a door43 metadata. door43 metadata must have attributes loaded
func (dm *Door43Metadata) APIURL() string {
	return fmt.Sprintf("%sapi/v1/catalog/entry/%s/%s/", setting.AppURL, dm.Repo.FullName(), dm.Ref)
}

// GetTarballURL get the tarball URL of the tag or branch
func (dm *Door43Metadata) GetTarballURL() string {
	if dm.RefType == "branch" {
		return fmt.Sprintf("%s/archive/%s.tar.gz", dm.Repo.HTMLURL(), dm.CommitSHA[0:10])
	}
	return fmt.Sprintf("%s/archive/%s.tar.gz", dm.Repo.HTMLURL(), dm.Ref)
}

// GetZipballURL get the zipball URL of the tag or branch
func (dm *Door43Metadata) GetZipballURL() string {
	if dm.RefType == "branch" {
		return fmt.Sprintf("%s/archive/%s.zip", dm.Repo.HTMLURL(), dm.CommitSHA[0:10])
	}
	return fmt.Sprintf("%s/archive/%s.zip", dm.Repo.HTMLURL(), dm.Ref)
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
	// Use CommitID because of race condition to a branch
	if dm.MetadataType == "rc" {
		return fmt.Sprintf("%s/raw/commit/%s/manifest.yaml", dm.Repo.HTMLURL(), dm.CommitSHA)
	}
	// so far this means we have a ts or tc metadata entry, but need to change for scripture burrito!
	return fmt.Sprintf("%s/raw/commit/%s/manifest.json", dm.Repo.HTMLURL(), dm.CommitSHA)
}

// GetMetadataJSONURL gets the json representation of the contents of the manifest.yaml file
func (dm *Door43Metadata) GetMetadataJSONURL() string {
	return fmt.Sprintf("%smetadata/", dm.APIURL())
}

// GetMetadataAPIContentsURL gets the metadata API contents URL of the manifest.yaml file
func (dm *Door43Metadata) GetMetadataAPIContentsURL() string {
	return fmt.Sprintf("%s/contents/manifest.yaml?ref=%s", dm.Repo.APIURL(), dm.Ref)
}

// GetGitTreesURL gets the git trees URL for a repo and branch or tag for all files
func (dm *Door43Metadata) GetGitTreesURL() string {
	if dm.RefType == "branch" {
		return fmt.Sprintf("%s/git/trees/%s?recursive=1&per_page=99999", dm.Repo.APIURL(), dm.CommitSHA[0:10])
	}
	return fmt.Sprintf("%s/git/trees/%s?recursive=1&per_page=99999", dm.Repo.APIURL(), dm.Ref)
}

// GetContentsURL gets the contents URL for a repo and branch or tag for all files
func (dm *Door43Metadata) GetContentsURL() string {
	if dm.RefType == "branch" {
		return fmt.Sprintf("%s/contents?ref=%s", dm.Repo.APIURL(), dm.CommitSHA[0:10])
	}
	return fmt.Sprintf("%s/contents?ref=%s", dm.Repo.APIURL(), dm.Ref)
}

// GetBooks get the books of the manifest
func (dm *Door43Metadata) GetBooks() []string {
	var books []string
	if len(dm.Ingredients) > 0 {
		for _, ing := range dm.Ingredients {
			books = append(books, ing.Identifier)
		}
	}
	return books
}

func (dm *Door43Metadata) GetAlignmentCounts() map[string]int {
	counts := map[string]int{}
	if len(dm.Ingredients) > 0 {
		for _, ing := range dm.Ingredients {
			if ing.AlignmentCount != nil {
				counts[ing.Identifier] = *ing.AlignmentCount
			}
		}
	}
	return counts
}

// IsDoor43MetadataExist returns true if door43 metadata with given release ID already exists.
func IsDoor43MetadataExist(repoID, releaseID int64) (bool, error) {
	return db.GetEngine(db.DefaultContext).Get(&Door43Metadata{RepoID: repoID, ReleaseID: releaseID})
}

// InsertDoor43Metadata inserts a door43 metadata
func InsertDoor43Metadata(dm *Door43Metadata) error {
	if id, err := db.GetEngine(db.DefaultContext).Insert(dm); err != nil {
		return err
	} else if id > 0 {
		dm.ID = id
		if err := dm.GetRepo(); err != nil {
			return err
		}
		if dm.ReleaseID > 0 {
			if err := system.CreateRepositoryNotice("Door43 Metadata created for repo: %s, tag: %s", dm.Repo.Name, dm.Ref); err != nil {
				return err
			}
		} else {
			if err := system.CreateRepositoryNotice("Door43 Metadata created for repo: %s, branch: %s", dm.Repo.Name, dm.Ref); err != nil {
				return err
			}
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
func UpdateDoor43MetadataCols(ctx context.Context, dm *Door43Metadata, cols ...string) error {
	return updateDoor43MetadataCols(ctx, dm, cols...)
}

func updateDoor43MetadataCols(ctx context.Context, dm *Door43Metadata, cols ...string) error {
	id, err := db.GetEngine(ctx).ID(dm.ID).Cols(cols...).Update(dm)
	if id > 0 && dm.ReleaseID > 0 {
		err := dm.GetRepo()
		if err != nil {
			return err
		}
		if err := system.CreateRepositoryNotice("Door43 Metadata updated for repo: %s, tag: %s", dm.Repo.Name, dm.Ref); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return err
}

// UpdateDoor43Metadata update a;ll door43 metadata
func UpdateDoor43Metadata(dm *Door43Metadata) error {
	return updateDoor43Metadata(db.GetEngine(db.DefaultContext), dm)
}

func updateDoor43Metadata(e db.Engine, dm *Door43Metadata) error {
	id, err := e.ID(dm.ID).AllCols().Update(dm)
	if id > 0 && dm.ReleaseID > 0 {
		err := dm.GetRepo()
		if err != nil {
			return err
		}
		if err := system.CreateRepositoryNotice("Door43 Metadata updated for repo: %s, tag: %s", dm.Repo.Name, dm.Ref); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return err
}

// GetDoor43MetadataByID returns door43 metadata with given ID.
func GetDoor43MetadataByID(id, repoID int64) (*Door43Metadata, error) {
	dm := new(Door43Metadata)
	has, err := db.GetEngine(db.DefaultContext).
		ID(id).
		Get(dm)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDoor43MetadataNotExist{id, repoID, ""}
	}
	return dm, nil
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

// GetLatestDooor43MetadataByRepoID returns the latest door43 metadata in the catalog by repoID, if canBePrerelease, a prerelease entry can match
func GetLatestDooor43MetadataByRepoID(repoID int64, canBePrerelease bool) (*Door43Metadata, error) {
	return getLatestDooor43MetadataByRepoID(db.DefaultContext, repoID, canBePrerelease)
}

func getLatestDooor43MetadataByRepoID(ctx context.Context, repoID int64, canBePrerelease bool) (*Door43Metadata, error) {
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
		return nil, ErrDoor43MetadataNotExist{0, repoID, ""}
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

// GetMostRecentDoor43MetadataByStage returns the most recent Door43Metadatas of a given stage for a repo
func GetMostRecentDoor43MetadataByStage(ctx context.Context, repoID int64, stage door43metadata.Stage) (*Door43Metadata, error) {
	return getMostRecentDoor43MetadataByStage(ctx, repoID, stage)
}

func getMostRecentDoor43MetadataByStage(ctx context.Context, repoID int64, stage door43metadata.Stage) (*Door43Metadata, error) {
	dm := &Door43Metadata{RepoID: repoID, Stage: stage}
	has, err := db.GetEngine(ctx).Desc("release_date_unix").Get(dm)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDoor43MetadataNotExist{0, repoID, ""}
	}
	return dm, nil
}

// GetDoor43MetadataByRepoIDAndReleaseID returns the metadata of a given release ID (0 = default branch).
func GetDoor43MetadataByRepoIDAndRef(ctx context.Context, repoID int64, ref string) (*Door43Metadata, error) {
	return getDoor43MetadataByRepoIDAndRef(db.GetEngine(ctx), repoID, ref)
}

func getDoor43MetadataByRepoIDAndRef(e db.Engine, repoID int64, ref string) (*Door43Metadata, error) {
	dm := &Door43Metadata{
		RepoID: repoID,
		Ref:    ref,
	}
	has, err := e.Get(dm)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDoor43MetadataNotExist{0, repoID, ref}
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
func DeleteDoor43MetadataByID(id, repoID int64) error {
	dm, err := GetDoor43MetadataByID(id, repoID)
	if err != nil || dm.RepoID != repoID {
		return err
	}
	return DeleteDoor43Metadata(dm)
}

// DeleteDoor43Metadata deletes a metadata from database by given ID.
func DeleteDoor43Metadata(dm *Door43Metadata) error {
	id, err := db.GetEngine(db.DefaultContext).Delete(dm)
	if id > 0 && dm.ReleaseID > 0 {
		if err := dm.GetRepo(); err != nil {
			return err
		} else if err := system.CreateRepositoryNotice("Door43 Metadata deleted for repo: %s, tag: %s", dm.Repo.Name, dm.Ref); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return err
}

// DeleteDoor43MetadataByRelease deletes a metadata from database by given repo ID and ref.
func DeleteDoor43MetadataByRepoIDAndRef(ctx context.Context, repoID int64, ref string) error {
	dm, err := GetDoor43MetadataByRepoIDAndRef(ctx, repoID, ref)
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
func DeleteAllDoor43MetadatasByRepoID(ctx context.Context, repoID int64) (int64, error) {
	return db.GetEngine(ctx).Delete(Door43Metadata{RepoID: repoID})
}

// GetReposForMetadata gets all the repos to process for metadata
func GetReposForMetadata(ctx context.Context) ([]*Repository, error) {
	var repos []*Repository
	err := db.GetEngine(ctx).
		Join("INNER", "user", "`user`.id = `repository`.owner_id").
		Where(builder.Eq{"`repository`.is_archived": 0}.And(builder.Eq{"`repository`.is_private": 0})).
		OrderBy("CASE WHEN `user`.lower_name = 'unfoldingword' THEN 0 " +
			"WHEN `user`.lower_name = 'door43-catalog' THEN 1 " +
			"WHEN `user`.lower_name LIKE '%_gl' THEN 2 " +
			"ELSE 3 END").
		OrderBy("`user`.type DESC").
		OrderBy("`user`.lower_name").
		OrderBy("`repository`.lower_name").
		Find(&repos)
	return repos, err
}

// GetRepoReleaseTagsForMetadata gets the releases tags for a repo used for getting metadata
func GetRepoReleaseTagsForMetadata(ctx context.Context, repoID int64) ([]string, error) {
	var releases []*Release
	err := db.GetEngine(ctx).
		Join("INNER", "repository", "`repository`.id = `release`.repo_id").
		Where(builder.Eq{"`release`.is_tag": 0}.And(builder.Eq{"`repository`.id": repoID})).
		OrderBy("`release`.created_unix").
		Find(&releases)
	if err != nil {
		return nil, err
	}

	tags := make([]string, len(releases))
	for idx, release := range releases {
		tags[idx] = release.TagName
	}

	return tags, nil
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
	ID     int64
	RepoID int64
	Ref    string
}

// IsErrDoor43MetadataNotExist checks if an error is a ErrDoor43MetadataNotExist.
func IsErrDoor43MetadataNotExist(err error) bool {
	_, ok := err.(ErrDoor43MetadataNotExist)
	return ok
}

func (err ErrDoor43MetadataNotExist) Error() string {
	return fmt.Sprintf("door43 metadata does not exist [id: %d, repo_id: %d, ref: %s]", err.ID, err.RepoID, err.Ref)
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
