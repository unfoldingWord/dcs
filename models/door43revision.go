// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"sort"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/unknwon/com"
	"xorm.io/builder"
	"xorm.io/xorm/schemas"
)

// Door43Revision represents the file revision of repository's file.
type Door43Revision struct {
	ID              int64                   `xorm:"pk autoincr"`
	RepoID          int64                   `xorm:"INDEX UNIQUE(n) NOT NULL"`
	Repo            *Repository             `xorm:"-"`
	ReleaseID       int64                   `xorm:"INDEX UNIQUE(n)"`
	Release         *Release                `xorm:"-"`
	TagName         string                  `xorm:"NOT NULL"`
	ProjectID       string
    Path            string                  `xorm:"NOT NULL"`
	Type            string
	Size            int64
	Revision        int64
	CutInVersion    string
	CutInVersionIteration int64
	CutOffVersion   string
	CutOffVersionIteration int64
	SHA string `xorm:"VARCHAR(40)"`
	ReleaseDateUnix timeutil.TimeStamp      `xorm:"NOT NULL"`
	CreatedUnix     timeutil.TimeStamp      `xorm:"INDEX created NOT NULL"`
	UpdatedUnix     timeutil.TimeStamp      `xorm:"INDEX updated"`
}

// GetRepo gets the repo associated with the door43 revision entry
func (dr *Door43Revision) GetRepo() error {
	return dr.getRepo(x)
}

func (dr *Door43Revision) getRepo(e Engine) error {
	if dr.Repo == nil {
		repo, err := GetRepositoryByID(dr.RepoID)
		if err != nil {
			return err
		}
		dr.Repo = repo
		if err := dr.Repo.getOwner(e); err != nil {
			return err
		}
	}
	return nil
}

// GetRelease gets the associated release of a door43 revision entry
func (dr *Door43Revision) GetRelease() error {
	return dr.getRelease(x)
}

func (dr *Door43Revision) getRelease(e Engine) error {
	if dr.Release == nil {
		rel, err := GetReleaseByID(dr.ReleaseID)
		if err != nil {
			if !IsErrDoor43RevisionNotExist(err) {
				return err
			}
			dr.Release = new(Release)
			dr.Release.ID = dr.ReleaseID
			dr.Release.TagName = dr.TagName
			dr.Release.CreatedUnix = dr.ReleaseDateUnix
			dr.Release.UpdatedUnix = dr.ReleaseDateUnix
			dr.Release.Title = dr.TagName
			dr.Release.LowerTagName = String.ToLower(dr.TagName)
			dr.Release.RepoID = dr.RepoID
		} else {
			dr.Release = rel
		}
		if err := dr.getRepo(e); err != nil {
			return err
		}
		dr.Release.Repo = dr.Repo
		return dr.Release.loadAttributes(e)
	}
	return nil
}

// LoadAttributes load repo and release attributes for a door43 revision
func (dr *Door43Revision) LoadAttributes() error {
	return dr.loadAttributes(x)
}

func (dr *Door43Revision) loadAttributes(e Engine) error {
	if err := dr.getRepo(e); err != nil {
		return err
	}
	if err := dr.getRelease(e); err != nil {
		return nil
	}
	return nil
}

// APIURLV5 the api url for a door43 revision. door43 revision must have attributes loaded
func (dr *Door43Revision) APIURLV5() string {
	return fmt.Sprintf("%sapi/catalog/v5/revision/%d", setting.AppURL, dr.ID)
}

// APIURLLatest the api url for a door43 revision. door43 revision must have attributes loaded
func (dr *Door43Revision) APIURLLatest() string {
	return fmt.Sprintf("%sapi/catalog/latest/revision/%d", setting.AppURL, dr.ID)
}

// GetTarballURL get the tarball URL of the release
func (dr *Door43Revision) GetTarballURL() string {
	return fmt.Sprintf("%s/archive/%s.tar.gz", dr.Repo.HTMLURL(), dr.TagName)
}

// GetZipballURL get the zipball URL of the tag
func (dr *Door43Revision) GetZipballURL() string {
	return fmt.Sprintf("%s/archive/%s.zip", dr.Repo.HTMLURL(), dr.TagName)
}

// GetReleaseURL get the URL the release API
func (dr *Door43Revision) GetReleaseURL() string {
	return fmt.Sprintf("%sapi/v1/repos/%s/releases/%d", setting.AppURL, dr.Repo.FullName(), dr.ReleaseID)
}

// GetRevisionAPIContentsURL gets the revision API contents URL of the manifest.yaml file
func (dr *Door43Revision) GetRevisionAPIContentsURL() string {
	return fmt.Sprintf("%s/contents/%s?ref=%s", dr.Repo.APIURL(), dr.Filepath, dr.TagName)
}

// RAWURL the url for a door43 revision on the web UI
func (dr *Door43Revision) RAWURL() string {
	return fmt.Sprintf("%s/raw/tag/%s/%s", dr.Repo.HTMLURL(), dr.TagName, dr.Filepath)
}

// IsDoor43RevisionExist returns true if door43 revision with given tag already exists.
func IsDoor43RevisionExist(repoID, tagName string) (bool, error) {
	return x.Get(&Door43Revision{RepoID: repoID, TagName: tagName})
}

// InsertDoor43Revision inserts a door43 revision
func InsertDoor43Revision(dr *Door43Revision) error {
	if id, err := x.Insert(dr); err != nil {
		return err
	} else if id > 0 {
		if err := dr.LoadAttributes(); err != nil {
			return err
		}
		if err := CreateRepositoryNotice("Door43 Revision created for repo: %s, tag: %s", dr.Repo.Name, dr.TagName); err != nil {
			return err
		}
	}
	return nil
}

// InsertDoor43RevisionsContext inserts door43 revisions
func InsertDoor43RevisionsContext(ctx DBContext, drs []*Door43Revision) error {
	_, err := ctx.e.Insert(drs)
	return err
}

// UpdateDoor43RevisionCols update door43 revision according special columns
func UpdateDoor43RevisionCols(dr *Door43Revision, cols ...string) error {
	return updateDoor43RevisionCols(x, dr, cols...)
}

func updateDoor43RevisionCols(e Engine, dr *Door43Revision, cols ...string) error {
	id, err := e.ID(dr.ID).Cols(cols...).Update(dr)
	if id > 0 {
		err := dr.LoadAttributes()
		if err != nil {
			return err
		}
		if err := CreateRepositoryNotice("Door43 Revision updated for repo: %s, tag: %s", dr.Repo.Name, dr.TagName); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return err
}

// GetDoor43RevisionByRepoIDAndTagName returns revision by given repo ID and tag name.
func GetDoor43RevisionByRepoIDAndTagName(repoID int64, tagName string) (*Door43Revision, error) {
	isExist, err := IsDoor43RevisionExist(repoID, tagName)
	if err != nil {
		return nil, err
	} else if !isExist {
		return nil, ErrDoor43RevisionNotExist{0, repoID, tagName}
	}

	dm := &Door43Revision{RepoID: repoID, ReleaseID: releaseID}
	_, err = x.Get(dm)
	return dm, err
}

// GetDoor43RevisionByID returns door43 revision with given ID.
func GetDoor43RevisionByID(id int64) (*Door43Revision, error) {
	rel := new(Door43Revision)
	has, err := x.
		ID(id).
		Get(rel)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDoor43RevisionNotExist{id, 0, 0}
	}

	return rel, nil
}

// FindDoor43RevisionsOptions describes the conditions to find door43 revisions
type FindDoor43RevisionsOptions struct {
	ListOptions
	ReleaseIDs []int64
}

func (opts *FindDoor43RevisionsOptions) toConds(repoID int64) builder.Cond {
	var cond = builder.NewCond()
	cond = cond.And(builder.Eq{"repo_id": repoID})

	if len(opts.ReleaseIDs) > 0 {
		cond = cond.And(builder.In("release_id", opts.ReleaseIDs))
	}
	return cond
}

// GetDoor43RevisionsByRepoID returns a list of door43 revisions of repository.
func GetDoor43RevisionsByRepoID(repoID int64, opts FindDoor43RevisionsOptions) ([]*Door43Revision, error) {
	sess := x.
		Desc("created_unix", "id").
		Where(opts.toConds(repoID))

	if opts.PageSize > 0 {
		sess = opts.setSessionPagination(sess)
	}

	dms := make([]*Door43Revision, 0, opts.PageSize)
	return dms, sess.Find(&dms)
}

// GetLatestCatalogRevisionByRepoID returns the latest door43 revision in the catalog by repoID, if canBePrerelease, a prerelease entry can match
func GetLatestCatalogRevisionByRepoID(repoID int64, canBePrerelease bool) (*Door43Revision, error) {
	return getLatestCatalogRevisionByRepoID(x, repoID, canBePrerelease)
}

func getLatestCatalogRevisionByRepoID(e Engine, repoID int64, canBePrerelease bool) (*Door43Revision, error) {
	cond := builder.NewCond().
		And(builder.Eq{"`door43_metadata`.repo_id": repoID}).
		And(builder.Eq{"`release`.is_tag": 0}).
		And(builder.Eq{"`release`.is_draft": 0})

	if !canBePrerelease {
		cond = cond.And(builder.Eq{"`release`.is_prerelease": 0})
	}

	dm := new(Door43Revision)
	has, err := e.
		Join("INNER", "release", "`release`.id = `door43_metadata`.release_id").
		Where(cond).
		Desc("`release`.created_unix", "`release`.id").
		Get(dm)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDoor43RevisionNotExist{0, repoID, ""}
	}

	return dm, dr.loadAttributes(e)
}

// GetDoor43RevisionsByRepoIDAndTagName returns a list of door43 revisions of repository according repoID and releaseIDs.
func GetDoor43RevisionsByRepoIDAndTagName(repoID int64, tagNames []string) (dms []*Door43Revision, err error) {
	err = x.In("tag_name", tagNames).
		Desc("created_unix").
		Find(&dms, Door43Revision{RepoID: repoID})
	return dms, err
}

// GetDoor43RevisionCountByRepoID returns the count of revisions of repository
func GetDoor43RevisionCountByRepoID(repoID int64, opts FindDoor43RevisionsOptions) (int64, error) {
	return x.Where(opts.toConds(repoID)).Count(&Door43Revision{})
}

// GetReleaseCount returns the count of releases of repository of the Door43Revision's stage
func (dr *Door43Revision) GetReleaseCount() (int64, error) {
	stageCond := GetStageCond(dr.Stage)
	return x.Join("LEFT", "release", "`release`.id = `door43_metadata`.release_id").
		Where(builder.And(builder.Eq{"`door43_metadata`.repo_id": dr.RepoID}, stageCond)).
		Count(&Door43Revision{})
}

// GetReleaseDateTime returns the ReleaseDateUnix time stamp as a RFC3339 date, e.g. 2006-01-02T15:04:05Z07:00
func (dr *Door43Revision) GetReleaseDateTime() string {
	return dr.ReleaseDateUnix.Format(time.RFC3339)
}

// GetDoor43RevisionByRepoIDAndReleaseID returns the revision of a given release ID (0 = default branch).
func GetDoor43RevisionByRepoIDAndTagName(repoID, tagName string) (*Door43Revision, error) {
	return getDoor43RevisionByRepoIDAndReleaseID(x, repoID, tagName)
}

func getDoor43RevisionByRepoIDAndTagName(e Engine, repoID, tagName string) (*Door43Revision, error) {
	dr := &Door43Revision{RepoID: repoID, TagName: tagName}
	has, err := e.Get(dr)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDoor43RevisionNotExist{0, repoID, tagName}
	}
	return dr, err
}

type door43RevisionSorter struct {
	drs []*Door43Revision
}

func (drs *door43RevisionSorter) Len() int {
	return len(drs.drs)
}

func (drs *door43RevisionSorter) Less(i, j int) bool {
	return drs.drs[i].ReleaseDateUnix > drs.drs[j].ReleaseDateUnix
}

func (dms *door43RevisionSorter) Swap(i, j int) {
	dms.dms[i], dms.dms[j] = dms.dms[j], dms.dms[i]
}

// SortDoorRevisions sorts door43 revisions by number of commits and created time.
func SortDoorRevisions(dms []*Door43Revision) {
	sorter := &door43RevisionSorter{dms: dms}
	sort.Sort(sorter)
}

// DeleteDoor43RevisionByID deletes a revision from database by given ID.
func DeleteDoor43RevisionByID(id int64) error {
	if dm, err := GetDoor43RevisionByID(id); err != nil {
		return err
	} else {
		return DeleteDoor43Revision(dm)
	}
}

// DeleteDoor43Revision deletes a revision from database by given ID.
func DeleteDoor43Revision(dr *Door43Revision) error {
	id, err := x.Delete(dm)
	if id > 0 {
		if err := dr.LoadAttributes(); err != nil {
			return err
		} else if err := CreateRepositoryNotice("Door43 Revision #%d deleted for repo: %s, tag: %s, path: %s", dr.ID, dr.Repo.Name, dr.TagName, dr.Path); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return err
}

// GetLastDoor43Revision gets the last Door43 Revision for path of a repo.
func (r *Repository) GetLastDoor43Revision(path string) (*Door43Revision, error) {
	return r.getLastRevision(x, path)
}

func (r *Repository) getLastDoor43Revision(e Engine, path string) (*Door43Revision, error) {
	dr, err := getLastDoor43RevisionByRepoID(e, r.ID, path)
	if err != nil && !IsErrDoor43RevisionNotExist(err) {
		return nil, err
	}
	return dr, nil
}

// GetLastDoor43RevisionByRepoID returns the last door43 revision for a path by repoID, nil if no revision yet
func GetLastDoor43RevisionByRepoID(repoID int64, path string) (*Door43Revision, error) {
	return getLastRevisionByRepoID(x, repoID, path)
}

func getLastDoor43RevisionByRepoID(e Engine, repoID int64, path string) (*Door43Revision, error) {
	cond := builder.NewCond().
		And(builder.Eq{"repo_id": repoID}).
		And(builder.Eq{"path": path})

	dr := new(Door43Metadata)
	_, err := e.
		Where(cond).
		Desc("revision").
		Get(dr)
	if err != nil {
		return nil, err
	}
	return dr, dr.loadAttributes(e)
}

/*** Error Structs & Functions ***/

// ErrDoor43RevisionAlreadyExist represents a "Door43RevisionAlreadyExist" kind of error.
type ErrDoor43RevisionAlreadyExist struct {
	ReleaseID int64
}

// IsErrDoor43RevisionAlreadyExist checks if an error is a ErrDoor43RevisionAlreadyExist.
func IsErrDoor43RevisionAlreadyExist(err error) bool {
	_, ok := err.(ErrDoor43RevisionAlreadyExist)
	return ok
}

func (err ErrDoor43RevisionAlreadyExist) Error() string {
	return fmt.Sprintf("Revision for release already exists [release: %d]", err.ReleaseID)
}

// ErrDoor43RevisionNotExist represents a "Door43RevisionNotExist" kind of error.
type ErrDoor43RevisionNotExist struct {
	ID        int64
	RepoID    int64
	TagName string
}

// IsErrDoor43RevisionNotExist checks if an error is a ErrDoor43RevisionNotExist.
func IsErrDoor43RevisionNotExist(err error) bool {
	_, ok := err.(ErrDoor43RevisionNotExist)
	return ok
}

func (err ErrDoor43RevisionNotExist) Error() string {
	return fmt.Sprintf("Door43Revision does not exist [id: %d, repo_id: %d, tag_name: %s]", err.ID, err.RepoID, err.TagName)
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


/*** INIT DB ***/

// InitDoor43Revision does some db management
func InitDoor43Revision() error {
	switch x.Dialect().URI().DBType {
	case schemas.MYSQL:
		_, err := x.Exec("ALTER TABLE `door43_metadata` MODIFY `metadata` JSON")
		if err != nil {
			return fmt.Errorf("Error changing door43_metadata revision column type: %v", err)
		}
	}
	return nil
}

/*** END INIT DB ***/
