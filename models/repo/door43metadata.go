// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"slices"
	"sort"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	"gitea.dev/models/system"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/dcs"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"xorm.io/builder"
)

/*** INIT DB ***/

// InitDoor43Metadata does some db management
func InitDoor43Metadata(ctx context.Context) error {
	switch setting.Database.Type {
	case "mysql":
		_, err := db.GetEngine(ctx).Exec("ALTER TABLE `door43_metadata` MODIFY `metadata` JSON")
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
	ID                  int64                       `xorm:"pk autoincr"`
	RepoID              int64                       `xorm:"INDEX UNIQUE(repo_ref) index(repo_stage_latest_date) NOT NULL"`
	Repo                *Repository                 `xorm:"-"`
	ReleaseID           int64                       `xorm:"NOT NULL"`
	Release             *Release                    `xorm:"-"`
	Ref                 string                      `xorm:"INDEX UNIQUE(repo_ref) NOT NULL"`
	RefType             string                      `xorm:"NOT NULL"`
	CommitSHA           string                      `xorm:"NOT NULL VARCHAR(40)"`
	Commit              *git.Commit                 `xorm:"-"`
	Stage               door43metadata.Stage        `xorm:"INDEX index(latest_stage) index(repo_stage_latest_date) NOT NULL"`
	MetadataType        string                      `xorm:"INDEX NOT NULL"`
	MetadataVersion     string                      `xorm:"NOT NULL"`
	Subject             string                      `xorm:"INDEX NOT NULL"`
	FlavorType          string                      `xorm:"INDEX NOT NULL"`
	Flavor              string                      `xorm:"INDEX NOT NULL"`
	Abbreviation        string                      `xorm:"INDEX NOT NULL"`
	Title               string                      `xorm:"NOT NULL"`
	Publisher           string                      `xorm:"NOT NULL"`
	Language            string                      `xorm:"INDEX NOT NULL"`
	LanguageTitle       string                      `xorm:"NOT NULL"`
	LanguageDirection   string                      `xorm:"NOT NULL"`
	LanguageIsGL        bool                        `xorm:"NOT NULL DEFAULT false"`
	ContentFormat       string                      `xorm:"NOT NULL"`
	CheckingLevel       int                         `xorm:"NOT NULL"`
	Ingredients         []*structs.Ingredient       `xorm:"JSON"`
	Relations           []*structs.Relation         `xorm:"JSON"`
	HasAudio            bool                        `xorm:"INDEX NOT NULL DEFAULT false"`
	HasVideo            bool                        `xorm:"INDEX NOT NULL DEFAULT false"`
	HasPDF              bool                        `xorm:"INDEX NOT NULL DEFAULT false"`
	HasStream           bool                        `xorm:"INDEX NOT NULL DEFAULT false"`
	HasOther            bool                        `xorm:"INDEX NOT NULL DEFAULT false"`
	IsLatestForStage    bool                        `xorm:"INDEX index(latest_stage) index(repo_stage_latest_date) NOT NULL DEFAULT false"`
	IsRepoMetadata      bool                        `xorm:"INDEX NOT NULL DEFAULT false"`
	Metadata            map[string]any              `xorm:"JSON MEDIUMTEXT"`
	ValidationError     *jsonschema.ValidationError `xorm:"JSON MEDIUMTEXT"`
	HealthcheckSeverity SeverityLevel               `xorm:"INDEX NULL DEFAULT NULL"`
	HealthcheckCounts   map[SeverityLevel]int       `xorm:"JSON"`
	HealthcheckTimeUnix timeutil.TimeStamp          `xorm:"NOT NULL DEFAULT 0"`
	ReleaseDateUnix     timeutil.TimeStamp          `xorm:"INDEX index(repo_stage_latest_date) NOT NULL"`
	CreatedUnix         timeutil.TimeStamp          `xorm:"INDEX created NOT NULL"`
	UpdatedUnix         timeutil.TimeStamp          `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(Door43Metadata))
}

// MetadataFileName returns the name of the file the metadata of this entry comes from
func (dm *Door43Metadata) MetadataFileName() string {
	switch dm.MetadataType {
	case "sb":
		return "metadata.json"
	case "tc", "ts":
		return "manifest.json"
	default:
		return "manifest.yaml"
	}
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

// CatalogValidationErrorsURL the api url for a catalog metadata. door43 metadata must have attributes loaded
func (dm *Door43Metadata) CatalogValidationErrorsURL() string {
	return setting.AppURL + "api/v1/catalog/validation/" + url.PathEscape(dm.Repo.OwnerName) + "/" + url.PathEscape(dm.Repo.Name) + "/" + url.PathEscape(dm.Ref)
}

// HealthcheckAPIURL the api url for this entry's full health check results. door43 metadata must have attributes loaded
func (dm *Door43Metadata) HealthcheckAPIURL() string {
	return setting.AppURL + "api/v1/repos/" + url.PathEscape(dm.Repo.OwnerName) + "/" + url.PathEscape(dm.Repo.Name) + "/healthcheck?ref=" + url.QueryEscape(dm.Ref)
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
func (dm *Door43Metadata) ReleaseCount(ctx context.Context) (int64, error) {
	stageCond := door43metadata.GetStageCond(dm.Stage)
	return db.GetEngine(ctx).Join("LEFT", "release", "`release`.id = `door43_metadata`.release_id").
		Where(builder.And(builder.Eq{"`door43_metadata`.repo_id": dm.RepoID}, stageCond)).
		Count(&Door43Metadata{})
}

// MetadataFilename the file name of the manifest or metadata file
func (dm *Door43Metadata) MetadataFilename() string {
	switch dm.MetadataType {
	case "rc":
		return "manifest.yaml"
	case "sb":
		return "metadata.json"
	case "tc", "ts":
		return "manifest.json"
	default:
		return ""
	}
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
	_ = dm.Repo.LoadLatestDMs(ctx)
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

// Door43MetadataAttachmentFlagCols are the columns set by DetermineAttachmentFlags,
// for use with UpdateDoor43MetadataCols.
var Door43MetadataAttachmentFlagCols = []string{"has_audio", "has_video", "has_pdf", "has_stream", "has_other"}

// DetermineAttachmentFlags sets the Has* content flags (HasAudio, HasVideo,
// HasPDF, HasStream, HasOther) based on the names of the attachments of the
// DM's release. All flags are false when there is no release (branch refs).
// files.json / links.json manifests are skipped since they are expanded into
// remote attachments and deleted by the door43 metadata service.
func (dm *Door43Metadata) DetermineAttachmentFlags(ctx context.Context) error {
	dm.HasAudio = false
	dm.HasVideo = false
	dm.HasPDF = false
	dm.HasStream = false
	dm.HasOther = false
	if dm.ReleaseID == 0 {
		return nil
	}
	var attachments []*Attachment
	if err := db.GetEngine(ctx).Where("release_id = ?", dm.ReleaseID).Find(&attachments); err != nil {
		return err
	}
	for _, attachment := range attachments {
		if dcs.IsJSONManifestAttachmentName(attachment.Name) {
			continue
		}
		// Rebuild the raw "name|url" value for remote attachments (AfterLoad
		// splits it) so streaming URLs are part of what gets classified.
		name := attachment.Name
		if attachment.BrowserDownloadURL != "" {
			name += "|" + attachment.BrowserDownloadURL
		}
		switch dcs.GetAttachmentContentType(name) {
		case dcs.AttachmentContentTypeAudio:
			dm.HasAudio = true
		case dcs.AttachmentContentTypeVideo:
			dm.HasVideo = true
		case dcs.AttachmentContentTypePDF:
			dm.HasPDF = true
		case dcs.AttachmentContentTypeStream:
			dm.HasStream = true
		default:
			dm.HasOther = true
		}
	}
	return nil
}

// AggregateMediaFlagsForRepo makes the latest-for-stage prod and preprod entries of a
// repo the authority on the media of ALL the repo's releases at their stage: each gets
// its has_* flags set to its own release's attachment flags OR-ed with the flags of
// every other entry of the repo at that stage. Non-latest entries always keep the flags
// of their own release (see the demotion reset in handleLatestStageDM), so deleting a
// release or removing an attachment corrects the aggregate on the next run. Branch
// (stage latest) entries have no release attachments and are not touched.
func AggregateMediaFlagsForRepo(ctx context.Context, repoID int64) error {
	for _, stage := range []door43metadata.Stage{door43metadata.StageProd, door43metadata.StagePreProd} {
		dm := &Door43Metadata{}
		has, err := db.GetEngine(ctx).
			Where(builder.Eq{"repo_id": repoID, "stage": stage, "is_latest_for_stage": true}).
			Get(dm)
		if err != nil {
			return err
		}
		if !has {
			continue
		}
		// Start from the latest release's own attachments (which also heals a stale
		// aggregate left on this row), then OR in the other entries of the stage.
		if err := dm.DetermineAttachmentFlags(ctx); err != nil {
			return err
		}
		var others struct {
			HasAudio  bool
			HasVideo  bool
			HasPDF    bool
			HasStream bool
			HasOther  bool
		}
		if _, err := db.GetEngine(ctx).
			SQL("SELECT MAX(has_audio) AS has_audio, MAX(has_video) AS has_video, MAX(has_pdf) AS has_pdf, MAX(has_stream) AS has_stream, MAX(has_other) AS has_other "+
				"FROM door43_metadata WHERE repo_id = ? AND stage = ? AND id <> ?", repoID, stage, dm.ID).
			Get(&others); err != nil {
			return err
		}
		dm.HasAudio = dm.HasAudio || others.HasAudio
		dm.HasVideo = dm.HasVideo || others.HasVideo
		dm.HasPDF = dm.HasPDF || others.HasPDF
		dm.HasStream = dm.HasStream || others.HasStream
		dm.HasOther = dm.HasOther || others.HasOther
		if err := UpdateDoor43MetadataCols(ctx, dm, Door43MetadataAttachmentFlagCols...); err != nil {
			return err
		}
	}
	return nil
}

// IsDoor43MetadataExist returns true if door43 metadata with given release ID already exists.
func IsDoor43MetadataExist(ctx context.Context, repoID, releaseID int64) (bool, error) {
	return db.GetEngine(ctx).Get(&Door43Metadata{RepoID: repoID, ReleaseID: releaseID})
}

// InsertDoor43Metadata inserts a door43 metadata
func InsertDoor43Metadata(ctx context.Context, dm *Door43Metadata) error {
	// dm.ValidationError = pruneValidationError(dm.ValidationError, 65535) // Adjust maxLength as needed
	if id, err := db.GetEngine(ctx).Insert(dm); err != nil {
		return err
	} else if id > 0 {
		dm.ID = id
		if err := dm.LoadRepo(ctx); err != nil {
			return err
		}
		// if dm.ReleaseID > 0 {
		// 	if err := system.CreateRepositoryNotice("Door43 Metadata created for repo: %s, tag: %s", dm.Repo.Name, dm.Ref); err != nil {
		// 		return err
		// 	}
		// } else {
		// 	if err := system.CreateRepositoryNotice("Door43 Metadata created for repo: %s, branch: %s", dm.Repo.Name, dm.Ref); err != nil {
		// 		return err
		// 	}
		// }
	}
	return nil
}

// InsertDoor43Metadatas inserts door43 metadatas
func InsertDoor43Metadatas(ctx context.Context, dms []*Door43Metadata) error {
	// for _, dm := range dms {
	// 	dm.ValidationError = pruneValidationError(dm.ValidationError, 65535)
	// }
	_, err := db.GetEngine(ctx).Insert(dms)
	return err
}

// UpdateDoor43MetadataCols update door43 metadata according special columns
func UpdateDoor43MetadataCols(ctx context.Context, dm *Door43Metadata, cols ...string) error {
	// dm.ValidationError = pruneValidationError(dm.ValidationError, 65535) // Adjust maxLength as needed
	id, err := db.GetEngine(ctx).ID(dm.ID).Cols(cols...).Update(dm)
	if id > 0 && dm.ReleaseID > 0 {
		err := dm.LoadRepo(ctx)
		if err != nil {
			return err
		}
		// if err := system.CreateRepositoryNotice("Door43 Metadata updated for repo: %s, tag: %s", dm.Repo.Name, dm.Ref); err != nil {
		// 	log.Error("CreateRepositoryNotice: %v", err)
		// }
	}
	return err
}

// UpdateDoor43Metadata update a;ll door43 metadata
func UpdateDoor43Metadata(ctx context.Context, dm *Door43Metadata) error {
	// dm.ValidationError = pruneValidationError(dm.ValidationError, 65535) // Adjust maxLength as needed
	id, err := db.GetEngine(ctx).ID(dm.ID).AllCols().Update(dm)
	if id > 0 && dm.ReleaseID > 0 {
		err := dm.LoadRepo(ctx)
		if err != nil {
			return err
		}
		// if err := system.CreateRepositoryNotice("Door43 Metadata updated for repo: %s, tag: %s", dm.Repo.Name, dm.Ref); err != nil {
		// 	log.Error("CreateRepositoryNotice: %v", err)
		// }
	}
	return err
}

// GetDoor43MetadataByID returns door43 metadata with given ID.
func GetDoor43MetadataByID(ctx context.Context, id int64) (*Door43Metadata, error) {
	dm := new(Door43Metadata)
	has, err := db.GetEngine(ctx).
		ID(id).
		Get(dm)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDoor43MetadataNotExist{id, 0, 0, ""}
	}
	return dm, nil
}

// GetMostRecentDoor43MetadataByStage returns the most recent Door43Metadatas of a given stage for a repo
func GetMostRecentDoor43MetadataByStage(ctx context.Context, repoID int64, stage door43metadata.Stage) (*Door43Metadata, error) {
	dm := &Door43Metadata{}
	has, err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": repoID}).
		And(builder.Eq{"stage": stage}).
		And(builder.IsNull{"validation_error"}).
		Desc("release_date_unix").
		Get(dm)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDoor43MetadataNotExist{0, repoID, 0, ""}
	}
	return dm, nil
}

// GetDoor43MetadataByRepoIDAndReleaseID returns the metadata of a given release ID.
func GetDoor43MetadataByRepoIDAndReleaseID(ctx context.Context, repoID, relID int64) (*Door43Metadata, error) {
	if repoID == 0 || relID == 0 {
		return nil, errors.New("must provide a repo ID and a release ID")
	}
	dm := &Door43Metadata{}
	has, err := db.GetEngine(ctx).
		Where(builder.Eq{"repo_id": repoID}).
		And(builder.Eq{"release_id": relID}).
		Get(dm)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDoor43MetadataNotExist{0, repoID, relID, ""}
	}
	return dm, nil
}

// GetDoor43MetadataByRepoIDAndRef returns the metadata of a given repo ID and ref.
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
		return nil, ErrDoor43MetadataNotExist{0, repoID, 0, ref}
	}
	return dm, nil
}

// GetDoor43MetadataLanguageInfo returns a map keyed by lowercased language code of language info
// ("language", "language_title", "language_direction", "language_is_gl",
// "alternate_names") gathered from the door43_metadata rows of the given languages,
// skipping rows with an empty language_title. The first row of a language provides its
// info, with the rows ordered lowest stage first (prod before preprod, etc.) and newest
// (highest ID) first within a stage, so the primary language_title comes from the newest
// production entry when one exists; the differing titles of any further rows are added
// to "alternate_names", and "language_is_gl" is true if any row has it true.
func GetDoor43MetadataLanguageInfo(ctx context.Context, langs []string) (map[string]map[string]any, error) {
	info := make(map[string]map[string]any, len(langs))
	if len(langs) == 0 {
		return info, nil
	}
	var dms []*Door43Metadata
	if err := db.GetEngine(ctx).
		Cols("language", "language_title", "language_direction", "language_is_gl").
		In("language", langs).
		And(builder.Neq{"language_title": ""}).
		GroupBy("language, language_title, language_direction, language_is_gl").
		OrderBy("MIN(stage) ASC, MAX(id) DESC").
		Find(&dms); err != nil {
		return nil, err
	}
	for _, dm := range dms {
		lowerLang := strings.ToLower(dm.Language)
		langInfo, ok := info[lowerLang]
		if !ok {
			info[lowerLang] = map[string]any{
				"language":           dm.Language,
				"language_title":     dm.LanguageTitle,
				"language_direction": dm.LanguageDirection,
				"language_is_gl":     dm.LanguageIsGL,
				"alternate_names":    []string{},
			}
			continue
		}
		if dm.LanguageIsGL {
			langInfo["language_is_gl"] = true
		}
		if dm.LanguageTitle != langInfo["language_title"].(string) {
			altNames := langInfo["alternate_names"].([]string)
			if !slices.Contains(altNames, dm.LanguageTitle) {
				langInfo["alternate_names"] = append(altNames, dm.LanguageTitle)
			}
		}
	}
	return info, nil
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

	// Batch-load the repos, releases, users (owners + release publishers) and
	// release attachments in a fixed number of queries. The per-entry
	// dm.LoadAttributes path costs ~5 queries per entry, which made unpaginated
	// catalog searches time out on large catalogs.
	repoIDSet := make(map[int64]struct{}, len(dms))
	releaseIDSet := make(map[int64]struct{}, len(dms))
	for _, dm := range dms {
		if dm.Repo == nil {
			repoIDSet[dm.RepoID] = struct{}{}
		}
		if dm.ReleaseID > 0 && dm.Release == nil {
			releaseIDSet[dm.ReleaseID] = struct{}{}
		}
	}

	repoMap := make(map[int64]*Repository, len(repoIDSet))
	if len(repoIDSet) > 0 {
		ids := make([]int64, 0, len(repoIDSet))
		for id := range repoIDSet {
			ids = append(ids, id)
		}
		var repos []*Repository
		if err := db.GetEngine(ctx).In("id", ids).Find(&repos); err != nil {
			return fmt.Errorf("find repos: %v", err)
		}
		for _, r := range repos {
			repoMap[r.ID] = r
		}
	}

	relMap := make(map[int64]*Release, len(releaseIDSet))
	if len(releaseIDSet) > 0 {
		ids := make([]int64, 0, len(releaseIDSet))
		for id := range releaseIDSet {
			ids = append(ids, id)
		}
		var rels []*Release
		if err := db.GetEngine(ctx).In("id", ids).Find(&rels); err != nil {
			return fmt.Errorf("find releases: %v", err)
		}
		for _, rel := range rels {
			relMap[rel.ID] = rel
		}
	}

	for _, dm := range dms {
		if dm.Repo == nil {
			dm.Repo = repoMap[dm.RepoID]
		}
		if dm.ReleaseID > 0 && dm.Release == nil {
			dm.Release = relMap[dm.ReleaseID]
		}
		if dm.Release != nil {
			dm.Release.Repo = dm.Repo
			dm.Release.Door43Metadata = dm
		}
	}

	// Owners and release publishers in one user query
	userIDSet := make(map[int64]struct{}, len(dms))
	for _, dm := range dms {
		if dm.Repo != nil && dm.Repo.Owner == nil {
			userIDSet[dm.Repo.OwnerID] = struct{}{}
		}
		if dm.Release != nil && dm.Release.Publisher == nil && dm.Release.PublisherID > 0 {
			userIDSet[dm.Release.PublisherID] = struct{}{}
		}
	}
	userMap := make(map[int64]*user_model.User, len(userIDSet))
	if len(userIDSet) > 0 {
		ids := make([]int64, 0, len(userIDSet))
		for id := range userIDSet {
			ids = append(ids, id)
		}
		var users []*user_model.User
		if err := db.GetEngine(ctx).In("id", ids).Find(&users); err != nil {
			return fmt.Errorf("find users: %v", err)
		}
		for _, u := range users {
			userMap[u.ID] = u
		}
	}
	rels := make([]*Release, 0, len(relMap))
	for _, dm := range dms {
		if dm.Repo != nil && dm.Repo.Owner == nil {
			dm.Repo.Owner = userMap[dm.Repo.OwnerID]
		}
		if dm.Release != nil {
			if dm.Release.Publisher == nil {
				if u := userMap[dm.Release.PublisherID]; u != nil {
					dm.Release.Publisher = u
				} else {
					dm.Release.Publisher = user_model.NewGhostUser()
				}
			}
			if dm.Release.Attachments == nil {
				rels = append(rels, dm.Release)
			}
		}
	}

	if len(rels) > 0 {
		if err := GetReleaseAttachments(ctx, rels...); err != nil {
			return fmt.Errorf("GetReleaseAttachments: %v", err)
		}
	}

	return nil
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
func DeleteDoor43MetadataByID(ctx context.Context, id int64) error {
	dm, err := GetDoor43MetadataByID(ctx, id)
	if err != nil {
		log.Error("GetDoor43MetadataByID: %v", err)
		return err
	}
	return DeleteDoor43Metadata(ctx, dm)
}

// DeleteDoor43Metadata deletes a metadata from database by given ID.
// The entry row is deleted first, inside a transaction with its health check issues, so
// an in-flight health check (StoreHealthcheckResults) serializes against the delete and
// never re-adds results for the deleted entry.
func DeleteDoor43Metadata(ctx context.Context, dm *Door43Metadata) error {
	var id int64
	err := db.WithTx(ctx, func(ctx context.Context) error {
		var err error
		id, err = db.GetEngine(ctx).Delete(dm)
		if err != nil {
			return err
		}
		if dm.ID > 0 {
			return DeleteDoor43HealthcheckIssuesByDMID(ctx, dm.ID)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if id > 0 && dm.ReleaseID > 0 {
		if err := dm.LoadRepo(ctx); err != nil {
			return err
		} else if err := system.CreateRepositoryNotice("Door43 Metadata deleted for repo: %s, tag: %s", dm.Repo.Name, dm.Ref); err != nil {
			log.Error("CreateRepositoryNotice: %v", err)
		}
	}
	return nil
}

// DeleteDoor43MetadataByRepoIDAndReleaseID deletes a metadata from database by given repo ID and release ID.
func DeleteDoor43MetadataByRepoIDAndReleaseID(ctx context.Context, repoID, relID int64) error {
	if repoID == 0 || relID == 0 {
		return errors.New("cannot delete door43_metadata with repo ID or release ID as 0")
	}
	dm, err := GetDoor43MetadataByRepoIDAndReleaseID(ctx, repoID, relID)
	if err != nil {
		if !IsErrDoor43MetadataNotExist(err) {
			return err
		}
		return nil
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).ID(dm.ID).Delete(dm); err != nil {
			return err
		}
		return DeleteDoor43HealthcheckIssuesByDMID(ctx, dm.ID)
	})
}

// DeleteDoor43MetadataByRepoIDAndRef deletes a metadata from database by given repo ID and ref.
func DeleteDoor43MetadataByRepoIDAndRef(ctx context.Context, repoID int64, ref string) error {
	dm, err := GetDoor43MetadataByRepoIDAndRef(ctx, repoID, ref)
	if err != nil {
		if !IsErrDoor43MetadataNotExist(err) {
			return err
		}
		return nil
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).ID(dm.ID).Delete(dm); err != nil {
			return err
		}
		return DeleteDoor43HealthcheckIssuesByDMID(ctx, dm.ID)
	})
}

// DeleteAllDoor43MetadatasByRepoID deletes all metadatas from database for a repo by given repo ID.
func DeleteAllDoor43MetadatasByRepoID(ctx context.Context, repoID int64) (int64, error) {
	var count int64
	err := db.WithTx(ctx, func(ctx context.Context) error {
		var err error
		count, err = db.GetEngine(ctx).Delete(Door43Metadata{RepoID: repoID})
		if err != nil {
			return err
		}
		return DeleteDoor43HealthcheckIssuesByRepoID(ctx, repoID)
	})
	return count, err
}

// DeleteDoor43MetadatasStaleRefs deletes entries (and their health check issues) whose
// ref is not in refs — the complete list of the repo's live release tags and branch
// names — and whose last update predates olderThan. The time guard spares entries
// created or updated after the caller captured its ref list (e.g. a branch pushed while
// a full reprocess was running). This is the eventual-consistency backstop for entries
// that outlive their ref: a ref deleted in the window between the delete notification
// and an in-flight insert leaves a row behind that this sweep removes on the next full
// reprocess (see docs/dcs/healthcheck.md).
func DeleteDoor43MetadatasStaleRefs(ctx context.Context, repoID int64, refs []string, olderThan timeutil.TimeStamp) (int64, error) {
	if len(refs) == 0 {
		// an empty ref list means the repo has no refs at all; that case is handled by
		// the repo-level cleanup (DeleteAllDoor43MetadatasByRepoID), not this sweep
		return 0, nil
	}
	var staleIDs []int64
	err := db.GetEngine(ctx).Table("door43_metadata").Cols("id").
		Where(builder.Eq{"repo_id": repoID}).
		And(builder.Lt{"updated_unix": olderThan}).
		NotIn("ref", refs).
		Find(&staleIDs)
	if err != nil || len(staleIDs) == 0 {
		return 0, err
	}
	var count int64
	err = db.WithTx(ctx, func(ctx context.Context) error {
		count, err = db.GetEngine(ctx).In("id", staleIDs).Delete(new(Door43Metadata))
		if err != nil {
			return err
		}
		_, err = db.GetEngine(ctx).In("dm_id", staleIDs).Delete(new(Door43HealthcheckIssue))
		return err
	})
	return count, err
}

// HasDefaultBranchConvertibleMetadata returns true if the repo has a door43_metadata row
// for its default branch with a metadata_type convertible to Scripture Burrito ("rc", "ts",
// or "tc"), regardless of validation errors.
func HasDefaultBranchConvertibleMetadata(ctx context.Context, repoID int64) (bool, error) {
	return db.GetEngine(ctx).
		Table("door43_metadata").
		Where(builder.Eq{"repo_id": repoID}.
			And(builder.Eq{"stage": int(door43metadata.StageLatest)}).
			And(builder.Eq{"is_latest_for_stage": true}).
			And(builder.In("metadata_type", "rc", "ts", "tc"))).
		Exist()
}

// GetReposQualifiedForSBConversion returns all repos that qualify for SB conversion:
// non-archived, non-private, default_branch = "master", have a door43_metadata row for their
// default branch with metadata_type = "rc", "ts", or "tc", and have at least one topic from
// the given list. Returns nil, nil if topics is empty.
func GetReposQualifiedForSBConversion(ctx context.Context, topics []string) ([]*Repository, error) {
	if len(topics) == 0 {
		return nil, nil
	}

	// Topics are always stored lowercase; lowercase inputs for safe matching
	lowerTopics := make([]any, len(topics))
	for i, t := range topics {
		lowerTopics[i] = strings.ToLower(t)
	}

	// Subquery: repo IDs that have at least one qualifying topic
	topicSubQ := builder.Select("repo_topic.repo_id").
		From("repo_topic").
		Join("INNER", "topic", "topic.id = repo_topic.topic_id").
		Where(builder.In("topic.name", lowerTopics...)).
		GroupBy("repo_topic.repo_id")

	// Subquery: repo IDs with a default-branch DM whose metadata_type is "rc", "ts", or "tc"
	dmSubQ := builder.Select("repo_id").
		From("door43_metadata").
		Where(builder.Eq{"stage": int(door43metadata.StageLatest)}.
			And(builder.Eq{"is_latest_for_stage": true}).
			And(builder.In("metadata_type", "rc", "ts", "tc")))

	cond := builder.Eq{"`repository`.default_branch": "master"}.
		And(builder.Eq{"`repository`.is_archived": 0}).
		And(builder.Eq{"`repository`.is_private": 0}).
		And(builder.In("`repository`.id", topicSubQ)).
		And(builder.In("`repository`.id", dmSubQ))

	var repos []*Repository
	err := db.GetEngine(ctx).
		Where(cond).
		OrderBy("`repository`.lower_name").
		Find(&repos)
	return repos, err
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
		Where(builder.Eq{"repo_id": repoID}).
		OrderBy("created_unix").
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
	RelID  int64
	Ref    string
}

// IsErrDoor43MetadataNotExist checks if an error is a ErrDoor43MetadataNotExist.
func IsErrDoor43MetadataNotExist(err error) bool {
	_, ok := err.(ErrDoor43MetadataNotExist)
	return ok
}

func (err ErrDoor43MetadataNotExist) Error() string {
	if err.Ref != "" {
		return fmt.Sprintf("door43 metadata does not exist [id: %d, repo_id: %d, ref: %s]", err.ID, err.RepoID, err.Ref)
	}
	return fmt.Sprintf("door43 metadata does not exist [id: %d, repo_id: %d, release_id: %d]", err.ID, err.RepoID, err.RelID)
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
