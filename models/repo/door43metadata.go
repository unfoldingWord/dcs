// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"html/template"
	"net/url"
	"sort"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"github.com/santhosh-tekuri/jsonschema/v5"

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
	ID                int64                       `xorm:"pk autoincr"`
	RepoID            int64                       `xorm:"INDEX UNIQUE(repo_ref) NOT NULL"`
	Repo              *Repository                 `xorm:"-"`
	ReleaseID         int64                       `xorm:"NOT NULL"`
	Release           *Release                    `xorm:"-"`
	Ref               string                      `xorm:"INDEX UNIQUE(repo_ref) NOT NULL"`
	RefType           string                      `xorm:"NOT NULL"`
	CommitSHA         string                      `xorm:"NOT NULL VARCHAR(40)"`
	Stage             door43metadata.Stage        `xorm:"INDEX NOT NULL"`
	MetadataType      string                      `xorm:"INDEX NOT NULL"`
	MetadataVersion   string                      `xorm:"NOT NULL"`
	Subject           string                      `xorm:"INDEX NOT NULL"`
	FlavorType        string                      `xorm:"INDEX NOT NULL"`
	Flavor            string                      `xorm:"INDEX NOT NULL"`
	Abbreviation      string                      `xorm:"INDEX NOT NULL"`
	Title             string                      `xorm:"NOT NULL"`
	Language          string                      `xorm:"INDEX NOT NULL"`
	LanguageTitle     string                      `xorm:"NOT NULL"`
	LanguageDirection string                      `xorm:"NOT NULL"`
	LanguageIsGL      bool                        `xorm:"NOT NULL DEFAULT false"`
	ContentFormat     string                      `xorm:"NOT NULL"`
	CheckingLevel     int                         `xorm:"NOT NULL"`
	Ingredients       []*structs.Ingredient       `xorm:"JSON"`
	IsLatestForStage  bool                        `xorm:"INDEX NOT NULL DEFAULT false"`
	IsRepoMetadata    bool                        `xorm:"INDEX NOT NULL DEFAULT false"`
	Metadata          map[string]interface{}      `xorm:"JSON"`
	ValidationError   *jsonschema.ValidationError `xorm:"JSON"`
	ReleaseDateUnix   timeutil.TimeStamp          `xorm:"NOT NULL"`
	CreatedUnix       timeutil.TimeStamp          `xorm:"INDEX created NOT NULL"`
	UpdatedUnix       timeutil.TimeStamp          `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(Door43Metadata))
}

// LoadRepo gets the repo associated with the door43 metadata entry
func (dm *Door43Metadata) LoadRepo(ctx context.Context) error {
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
func (dm *Door43Metadata) LoadRelease(ctx context.Context) error {
	if dm.ReleaseID > 0 && dm.Release == nil {
		rel, err := GetReleaseByID(ctx, dm.ReleaseID)
		if err != nil {
			return err
		}
		dm.Release = rel
	}
	if dm.Release != nil {
		dm.Release.Door43Metadata = dm
		dm.Release.Repo = dm.Repo
		if err := dm.Release.LoadAttributes(ctx); err != nil {
			log.Warn("LoadRelease - calling dm.Release.loadAttributes Error: %v\n", err)
			return err
		}
	}
	return nil
}

// LoadAttributes load repo and release attributes for a door43 metadata
func (dm *Door43Metadata) LoadAttributes(ctx context.Context) error {
	if err := dm.LoadRepo(ctx); err != nil {
		return err
	}
	if dm.ReleaseID > 0 {
		if err := dm.LoadRelease(ctx); err != nil {
			log.Error("LoadRelease: %v", err)
			return nil
		}
	}
	return nil
}

// CatalogEntryURL the api url for a door43 metadata. door43 metadata must have attributes loaded
func (dm *Door43Metadata) CatalogEntryURL() string {
	return setting.AppURL + "api/v1/catalog/entry/" + url.PathEscape(dm.Repo.OwnerName) + "/" + url.PathEscape(dm.Repo.Name) + "/" + url.PathEscape(dm.Ref)
}

// CatalogMetadataJSONURL the api url for a catalog metadata. door43 metadata must have attributes loaded
func (dm *Door43Metadata) CatalogMetatadataJSONURL() string {
	return setting.AppURL + "api/v1/catalog/metadata/" + url.PathEscape(dm.Repo.OwnerName) + "/" + url.PathEscape(dm.Repo.Name) + "/" + url.PathEscape(dm.Ref)
}

// CatalogValidationErrorURL the api url for a catalog metadata. door43 metadata must have attributes loaded
func (dm *Door43Metadata) CatalogValidationErrorURL() string {
	return setting.AppURL + "api/v1/catalog/validation/" + url.PathEscape(dm.Repo.OwnerName) + "/" + url.PathEscape(dm.Repo.Name) + "/" + url.PathEscape(dm.Ref)
}

// TarballURL the tarball URL of the tag or branch
func (dm *Door43Metadata) TarballURL() string {
	if dm.RefType == "branch" {
		return fmt.Sprintf("%s/archive/%s.tar.gz", dm.Repo.HTMLURL(), dm.CommitSHA[0:10])
	}
	return fmt.Sprintf("%s/archive/%s.tar.gz", dm.Repo.HTMLURL(), dm.Ref)
}

// ZipballURL the zipball URL of the tag or branch
func (dm *Door43Metadata) ZipballURL() string {
	if dm.RefType == "branch" {
		return fmt.Sprintf("%s/archive/%s.zip", dm.Repo.HTMLURL(), dm.CommitSHA[0:10])
	}
	return fmt.Sprintf("%s/archive/%s.zip", dm.Repo.HTMLURL(), dm.Ref)
}

// ReleaseURL the URL the release API
func (dm *Door43Metadata) ReleaseURL(ctx context.Context) string {
	if dm.ReleaseID > 0 {
		if dm.Release != nil {
			return dm.Release.APIURL()
		}
		if err := dm.LoadRepo(ctx); err == nil {
			return fmt.Sprintf("%sapi/v1/repos/%s/releases/%d", setting.AppURL, dm.Repo.FullName(), dm.ReleaseID)
		}
	}
	return ""
}

// RawMetadataFileURL the url to the raw manifest or metadata file
func (dm *Door43Metadata) RawMetadataFileURL() string {
	// Use CommitID because of race condition to a branch
	return fmt.Sprintf("%s/raw/commit/%s/%s", dm.Repo.HTMLURL(), dm.CommitSHA, dm.MetadataFilename())
}

// MetadataTypeTitle the metadata type title
func (dm *Door43Metadata) MetadataTypeTitle() string {
	switch dm.MetadataType {
	case "ts":
		return "translationStudio"
	case "tc":
		return "translationCore"
	case "rc":
		return "Resource Container"
	case "sb":
		return "Scripture Burrito"
	default:
		return dm.MetadataType
	}
}

// MetadataTypeIcon the metadata type icon
func (dm *Door43Metadata) MetadataTypeIcon() string {
	switch dm.MetadataType {
	case "rc":
		return "rc.png"
	case "ts":
		return "ts.png"
	case "tc":
		return "tc.png"
	case "sb":
		return "sb.png"
	default:
		return "uw.png"
	}
}

// MetadataJSONString the JSON in string format of a map
func (dm *Door43Metadata) MetadataJSONString() string {
	json, _ := json.MarshalIndent(dm.Metadata, "", "    ")
	return string(json)
}

// ValidationErrorJSONString the JSON in string format of a map
func (dm *Door43Metadata) ValidationErrorJSONString() string {
	if dm.ValidationError == nil {
		return ""
	}
	json, _ := json.MarshalIndent(dm.ValidationError.BasicOutput(), "", "    ")
	return string(json)
}

// MetadataAPIContentsURL the metadata API contents URL of the manifest or metadata file
func (dm *Door43Metadata) MetadataAPIContentsURL() string {
	return fmt.Sprintf("%s/contents/%s?ref=%s", dm.Repo.APIURL(), dm.MetadataFilename(), dm.Ref)
}

// StageStr the string representation of a stage int
func (dm *Door43Metadata) StageStr() string {
	return door43metadata.StageToStringMap[dm.Stage]
}

// GitTreesURL the git trees URL for a repo and branch or tag for all files
func (dm *Door43Metadata) GitTreesURL() string {
	if dm.RefType == "branch" {
		return fmt.Sprintf("%s/git/trees/%s?recursive=1&per_page=99999", dm.Repo.APIURL(), dm.CommitSHA[0:10])
	}
	return fmt.Sprintf("%s/git/trees/%s?recursive=1&per_page=99999", dm.Repo.APIURL(), dm.Ref)
}

// ContentsURL the contents URL for a repo and branch or tag for all files
func (dm *Door43Metadata) ContentsURL() string {
	if dm.RefType == "branch" {
		return fmt.Sprintf("%s/contents?ref=%s", dm.Repo.APIURL(), dm.CommitSHA[0:10])
	}
	return fmt.Sprintf("%s/contents?ref=%s", dm.Repo.APIURL(), dm.Ref)
}

// IngredientsIdentifierList the identifiers of the igredients and returns them as a list of strings
func (dm *Door43Metadata) IngredientsIdentifierList() []string {
	var ids []string
	if len(dm.Ingredients) > 0 {
		for _, ing := range dm.Ingredients {
			ids = append(ids, ing.Identifier)
		}
	}
	return ids
}

// IngredientsAsString the integredients of the repo and returns the identifiers as a comma-delimited string
func (dm *Door43Metadata) IngredientsAsString() string {
	ids := dm.IngredientsIdentifierList()
	return strings.Join(ids, ", ")
}

// AlignmenetCounts the alignment counts of all the books of a book repo
func (dm *Door43Metadata) AlignmentCounts() map[string]int {
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

// ReleaseCount the count of releases of repository of the Door43Metadata's stage
func (dm *Door43Metadata) ReleaseCount() (int64, error) {
	stageCond := door43metadata.GetStageCond(dm.Stage)
	return db.GetEngine(db.DefaultContext).Join("LEFT", "release", "`release`.id = `door43_metadata`.release_id").
		Where(builder.And(builder.Eq{"`door43_metadata`.repo_id": dm.RepoID}, stageCond)).
		Count(&Door43Metadata{})
}

// MetadataFilename the file name of the manifest or metadata file
func (dm *Door43Metadata) MetadataFilename() string {
	if dm.MetadataType == "rc" {
		return "manifest.yaml"
	} else if dm.MetadataType == "sb" {
		return "metadata.json"
	} else if dm.MetadataType == "tc" || dm.MetadataType == "ts" {
		return "manifest.json"
	}
	return ""
}

// ValidationErrorAsTemplateHTML the validation error object as a template.HTML
func (dm *Door43Metadata) ValidationErrorAsTemplateHTML() *template.HTML {
	if dm.ValidationError == nil {
		return nil
	}
	html := template.HTML(convertValidationErrorToHTML(dm.ValidationError, nil))
	return &html
}

func convertValidationErrorToHTML(valErr, parentErr *jsonschema.ValidationError) string {
	if valErr == nil {
		return ""
	}
	var label string
	var html string
	if parentErr == nil {
		html = fmt.Sprintf("<strong>Invalid:</strong> %s\n", strings.TrimSuffix(valErr.Message, "#"))
		html += "<ul>\n"
		if len(valErr.Causes) > 0 {
			label += "<strong>&lt;root&gt;:</strong>\n"
		}
	} else {
		loc := ""
		if valErr.InstanceLocation != "" {
			loc = strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(valErr.InstanceLocation, parentErr.InstanceLocation), "/"), "/", ".")
			if loc != "" {
				loc = fmt.Sprintf("<strong>%s:</strong> ", strings.TrimPrefix(loc, "/"))
			}
		}
		msg := ""
		if valErr.Message != "if-else failed" && valErr.Message != "if-then failed" {
			msg = valErr.Message
		}
		label = loc + msg
	}
	sort.Slice(valErr.Causes, func(i, j int) bool { return valErr.Causes[i].InstanceLocation < valErr.Causes[j].InstanceLocation })
	if label != "" {
		html += "<ul><li>" + label + "</li>"
	}
	for _, cause := range valErr.Causes {
		html += convertValidationErrorToHTML(cause, valErr)
	}
	if label != "" {
		html += "</ul>\n"
	}
	return html
}

// CopyEmptyPropertiesFromRepoDM copies all general properties from the main repo DM if empty
func (dm *Door43Metadata) CopyEmptyPropertiesFromRepoDM(ctx context.Context) {
	if dm.Repo == nil {
		return
	}
	dm.Repo.LoadLatestDMs(ctx)
	if dm.Repo.RepoDM == nil {
		return
	}
	if dm.Title == "" {
		dm.Title = dm.Repo.RepoDM.Title
	}
	if dm.Abbreviation == "" {
		dm.Abbreviation = dm.Repo.RepoDM.Abbreviation
	}
	if dm.MetadataType == "" {
		dm.MetadataType = dm.Repo.RepoDM.MetadataType
	}
	if dm.MetadataVersion == "" {
		dm.MetadataVersion = dm.Repo.RepoDM.MetadataVersion
	}
	if dm.Subject == "" {
		dm.Subject = dm.Repo.RepoDM.Subject
	}
	if dm.FlavorType == "" {
		dm.FlavorType = dm.Repo.RepoDM.FlavorType
	}
	if dm.Flavor == "" {
		dm.Flavor = dm.Repo.RepoDM.Flavor
	}
	if dm.Resource == "" {
		dm.Resource = dm.Repo.RepoDM.Resource
	}
	if dm.Abbreviation == "" {
		dm.Abbreviation = dm.Repo.RepoDM.Abbreviation
	}
	if dm.Title == "" {
		dm.Title = dm.Repo.RepoDM.Title
	}
	if dm.Language == "" {
		dm.Language = dm.Repo.RepoDM.Language
	}
	if dm.LanguageTitle == "" {
		dm.LanguageTitle = dm.Repo.RepoDM.LanguageTitle
	}
	if dm.LanguageDirection == "" {
		dm.LanguageDirection = dm.Repo.RepoDM.LanguageDirection
	}
	if !dm.LanguageIsGL {
		dm.LanguageIsGL = dm.Repo.RepoDM.LanguageIsGL
	}
	if dm.ContentFormat == "" {
		dm.ContentFormat = dm.Repo.RepoDM.ContentFormat
	}
}

// IsDoor43MetadataExist returns true if door43 metadata with given release ID already exists.
func IsDoor43MetadataExist(ctx context.Context, repoID, releaseID int64) (bool, error) {
	return db.GetEngine(ctx).Get(&Door43Metadata{RepoID: repoID, ReleaseID: releaseID})
}

// InsertDoor43Metadata inserts a door43 metadata
func InsertDoor43Metadata(ctx context.Context, dm *Door43Metadata) error {
	if id, err := db.GetEngine(ctx).Insert(dm); err != nil {
		return err
	} else if id > 0 {
		dm.ID = id
		if err := dm.LoadRepo(ctx); err != nil {
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

// InsertDoor43Metadatas inserts door43 metadatas
func InsertDoor43Metadatas(ctx context.Context, dms []*Door43Metadata) error {
	_, err := db.GetEngine(ctx).Insert(dms)
	return err
}

// UpdateDoor43MetadataCols update door43 metadata according special columns
func UpdateDoor43MetadataCols(ctx context.Context, dm *Door43Metadata, cols ...string) error {
	id, err := db.GetEngine(ctx).ID(dm.ID).Cols(cols...).Update(dm)
	if id > 0 && dm.ReleaseID > 0 {
		err := dm.LoadRepo(ctx)
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
func UpdateDoor43Metadata(ctx context.Context, dm *Door43Metadata) error {
	id, err := db.GetEngine(ctx).ID(dm.ID).AllCols().Update(dm)
	if id > 0 && dm.ReleaseID > 0 {
		err := dm.LoadRepo(ctx)
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
func GetDoor43MetadataByID(ctx context.Context, id, repoID int64) (*Door43Metadata, error) {
	dm := new(Door43Metadata)
	has, err := db.GetEngine(ctx).
		ID(id).
		Get(dm)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDoor43MetadataNotExist{id, repoID, ""}
	}
	return dm, nil
}

// GetMostRecentDoor43MetadataByStage returns the most recent Door43Metadatas of a given stage for a repo
func GetMostRecentDoor43MetadataByStage(ctx context.Context, repoID int64, stage door43metadata.Stage) (*Door43Metadata, error) {
	dm := &Door43Metadata{}
	has, err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": repoID}).
		And(builder.Eq{"stage": stage}).
		And(builder.Eq{"validation_error": nil}).
		Desc("release_date_unix").
		Get(dm)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDoor43MetadataNotExist{0, repoID, ""}
	}
	return dm, nil
}

// GetDoor43MetadataByRepoIDAndReleaseID returns the metadata of a given release ID (0 = default branch).
func GetDoor43MetadataByRepoIDAndRef(ctx context.Context, repoID int64, ref string) (*Door43Metadata, error) {
	dm := &Door43Metadata{}
	has, err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": repoID}).
		And(builder.Eq{"ref": ref}).
		Get(dm)
	if err != nil {
		return nil, err
	}
	if !has {
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
func (dms Door43MetadataList) LoadAttributes(ctx context.Context) error {
	if len(dms) == 0 {
		return nil
	}
	var lastErr error
	for _, dm := range dms {
		if err := dm.LoadAttributes(ctx); err != nil && lastErr == nil {
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
func DeleteDoor43MetadataByID(ctx context.Context, id, repoID int64) error {
	dm, err := GetDoor43MetadataByID(ctx, id, repoID)
	if err != nil || dm.RepoID != repoID {
		return err
	}
	return DeleteDoor43Metadata(ctx, dm)
}

// DeleteDoor43Metadata deletes a metadata from database by given ID.
func DeleteDoor43Metadata(ctx context.Context, dm *Door43Metadata) error {
	id, err := db.GetEngine(ctx).Delete(dm)
	if id > 0 && dm.ReleaseID > 0 {
		if err := dm.LoadRepo(ctx); err != nil {
			return err
		} else if err := system.CreateRepositoryNotice("Door43 Metadata deleted for repo: %s, tag: %s", dm.Repo.Name, dm.Ref); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return err
}

// DeleteDoor43MetadataByRepoRef deletes a metadata from database by given repo and ref.
func DeleteDoor43MetadataByRepoRef(ctx context.Context, repo *Repository, ref string) error {
	dm, err := GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref)
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
		Where(builder.Eq{"`repository`.id": repoID}).
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
