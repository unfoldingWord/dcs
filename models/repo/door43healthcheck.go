// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/json"
)

// SeverityLevel represents the level of severity or concern for a health check
type SeverityLevel int

const (
	SeverityLevelSuccess SeverityLevel = iota + 1 // 1
	SeverityLevelInfo                             // 2
	SeverityLevelWarning                          // 3
	SeverityLevelError                            // 4
)

// SeverityLevelMap map from string to SeverityLevel (int)
var SeverityLevelMap = map[string]SeverityLevel{
	"error":   SeverityLevelError,
	"warning": SeverityLevelWarning,
	"info":    SeverityLevelInfo,
	"success": SeverityLevelSuccess,
}

// SeverityLevelToStringMap map from SeverityLevel (int) to string
var SeverityLevelToStringMap = map[SeverityLevel]string{
	SeverityLevelError:   "error",
	SeverityLevelWarning: "warning",
	SeverityLevelInfo:    "info",
	SeverityLevelSuccess: "success",
}

// String returns string representation of a SeverityLevel (int)
func (sl SeverityLevel) String() string {
	return SeverityLevelToStringMap[sl]
}

// IsHealthy returns true if the level means the entry passed its health check, warnings allowed.
// A zero level means the entry was never checked, so it is not considered healthy.
func (sl SeverityLevel) IsHealthy() bool {
	return sl >= SeverityLevelSuccess && sl <= SeverityLevelWarning
}

// IsHealthyWithoutWarnings returns true if the entry passed its health check with no warnings.
func (sl SeverityLevel) IsHealthyWithoutWarnings() bool {
	return sl == SeverityLevelSuccess || sl == SeverityLevelInfo
}

func (sl *SeverityLevel) UnmarshalText(text []byte) error {
	*sl = SeverityLevelMap[strings.ToLower(string(text))]
	return nil
}

func (sl *SeverityLevel) MarshalText() ([]byte, error) {
	return []byte(sl.String()), nil
}

func (sl *SeverityLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(sl.String())
}

// IssueCode represents a specific issue type for a health check
type IssueCode string

// IssueCode values
const (
	IssueCodeNoMetadata             IssueCode = "no_metadata"                  // no metadata found for the repository
	IssueCodeMetadataInvalid        IssueCode = "metadata_invalid"             // metadata is invalid
	IssueCodeRelation               IssueCode = "relation_lang_invalid"        // relation not valid
	IssueCodePublisher              IssueCode = "publisher_has_uw"             // publisher empty or is 'unfoldingWord'
	IssueCodeTitle                  IssueCode = "title_has_uw"                 // title is empty or still contains 'unfoldingWord'
	IssueCodeIdentifier             IssueCode = "abbreviation_invalid"         // abbreviation is not valid for given subject
	IssueCodeLanguage               IssueCode = "language_is_en"               // language is empty or still English
	IssueCodeIngredientTitle        IssueCode = "ingredient_title_is_en"       // ingredient title is empty or still English
	IssueCodeIngredientMissing      IssueCode = "ingredient_missing"           // ingredient's path is missing
	IssueCodeIngredientEmpty        IssueCode = "ingredient_empty"             // ingredient's file is empty
	IssueCodeReleaseNeeded          IssueCode = "release_needed"               // a release is needed for the resource
	IssueCodeRelationMissing        IssueCode = "tn_relation_missing"          // TN relation is missing
	IssueCodeOrigLangVersionMissing IssueCode = "tn_orig_lang_version_missing" // TN orig lang version is missing
	IssueCodeOBSStoryMissing        IssueCode = "obs_story_missing"            // OBS story is missing
	IssueCodeOBSStoryTitleMissing   IssueCode = "obs_story_invalid"            // OBS story title is missing
	IssueCodeOBSWrongFrameCount     IssueCode = "obs_story_wrong_frame_count"  // OBS story has wrong frame count
	IssueCodeOBSBibleRefenceMissing IssueCode = "obs_bible_reference_missing"  // OBS bible reference is missing from story
	IssueCodeUSFMInvalid            IssueCode = "usfm_invalid"                 // USFM file is missing or not valid USFM
	IssueCodeUSFMNoAlignment        IssueCode = "usfm_no_alignment"            // USFM file has no alignment data
	IssueCodeSBIngredientMismatch   IssueCode = "sb_ingredient_mismatch"       // SB ingredient entry doesn't match the file in the repo
)

// HealthcheckGroupedIssues groups health check issues by issue code
type HealthcheckGroupedIssues struct {
	Issues               map[IssueCode][]*Door43HealthcheckIssue `json:"issues"`
	OverallSeverityLevel SeverityLevel                           `json:"overall_severity_level"`
	SeverityLevelCount   SeverityLevelCount                      `json:"severity_level_count"`
	MetadataType         string                                  `json:"-"`
	Subject              string                                  `json:"-"`
}

// SeverityLevelCount is a map of severity level to count
type SeverityLevelCount map[SeverityLevel]int

func (slc SeverityLevelCount) MarshalJSON() ([]byte, error) {
	result := make(map[string]int)
	for level, count := range slc {
		result[level.String()] = count
	}
	return json.Marshal(result)
}

// NewHealthcheckGroupedIssues creates a new HealthcheckGroupedIssues for a given metadata type and subject.
// Only issue codes applicable to the metadata type and subject are initialized.
func NewHealthcheckGroupedIssues(metadataType, subject string, issues []*Door43HealthcheckIssue) *HealthcheckGroupedIssues {
	hgi := &HealthcheckGroupedIssues{
		Issues:               make(map[IssueCode][]*Door43HealthcheckIssue),
		OverallSeverityLevel: SeverityLevelSuccess,
		SeverityLevelCount:   make(map[SeverityLevel]int),
		MetadataType:         metadataType,
		Subject:              subject,
	}
	for _, code := range IssueCodesFor(metadataType, subject) {
		hgi.Issues[code] = []*Door43HealthcheckIssue{}
	}
	for level := SeverityLevelSuccess; level <= SeverityLevelError; level++ {
		hgi.SeverityLevelCount[level] = 0
	}
	for _, issue := range issues {
		hgi.AddIssue(issue)
	}
	return hgi
}

// AddIssue adds an issue to the grouped issues
func (hgi *HealthcheckGroupedIssues) AddIssue(issue *Door43HealthcheckIssue) {
	hgi.Issues[issue.IssueCode] = append(hgi.Issues[issue.IssueCode], issue)
	if issue.SeverityLevel > hgi.OverallSeverityLevel {
		hgi.OverallSeverityLevel = issue.SeverityLevel
	}
	hgi.SeverityLevelCount[issue.SeverityLevel]++
}

// GetIssues returns all issues for a given issue code
func (hgi *HealthcheckGroupedIssues) GetIssues(issueCode IssueCode) []*Door43HealthcheckIssue {
	return hgi.Issues[issueCode]
}

// GetOrder returns the applicable issue codes for this healthcheck's metadata type and subject
func (hgi *HealthcheckGroupedIssues) GetOrder() []IssueCode {
	return IssueCodesFor(hgi.MetadataType, hgi.Subject)
}

// Issue code lists by metadata type and subject applicability

// commonIssueCodes are checked for all metadata types (rc, ts, tc, sb)
var commonIssueCodes = []IssueCode{
	IssueCodeNoMetadata,
	IssueCodeMetadataInvalid,
	IssueCodeTitle,
	IssueCodeLanguage,
	IssueCodeIngredientTitle,
	IssueCodeIngredientMissing,
	IssueCodeIngredientEmpty,
	IssueCodeReleaseNeeded,
}

// rcIssueCodes are only checked for Resource Container (rc) repos
var rcIssueCodes = []IssueCode{
	IssueCodePublisher,
	IssueCodeIdentifier,
	IssueCodeRelation,
	IssueCodeRelationMissing,
}

// tcIssueCodes are only checked for translationCore (tc) repos
var tcIssueCodes = []IssueCode{
	IssueCodeUSFMInvalid,
	IssueCodeUSFMNoAlignment,
}

// sbIssueCodes are only checked for Scripture Burrito (sb) repos
var sbIssueCodes = []IssueCode{
	IssueCodeSBIngredientMismatch,
}

// obsIssueCodes are only checked for Open Bible Stories
var obsIssueCodes = []IssueCode{
	IssueCodeOBSStoryMissing,
	IssueCodeOBSStoryTitleMissing,
	IssueCodeOBSWrongFrameCount,
	IssueCodeOBSBibleRefenceMissing,
}

// tnIssueCodes are only checked for TSV Translation Notes
var tnIssueCodes = []IssueCode{
	IssueCodeOrigLangVersionMissing,
}

// IssueCodesFor returns the issue codes applicable to the given metadata type and subject
func IssueCodesFor(metadataType, subject string) []IssueCode {
	codes := append([]IssueCode{}, commonIssueCodes...)
	switch metadataType {
	case "rc":
		codes = append(codes, rcIssueCodes...)
		switch subject {
		case "Open Bible Stories":
			codes = append(codes, obsIssueCodes...)
		case "TSV Translation Notes":
			codes = append(codes, tnIssueCodes...)
		}
	case "tc":
		codes = append(codes, tcIssueCodes...)
	case "sb":
		codes = append(codes, sbIssueCodes...)
	}
	return codes
}

var IssueCodeNegatives = map[IssueCode]string{
	IssueCodeNoMetadata:             "No metadata found for the repository",
	IssueCodeMetadataInvalid:        "Invalid metadata; file does not match schema",
	IssueCodeRelation:               "Relation is not the language of the resource",
	IssueCodePublisher:              "Publisher is still 'unfoldingWord'",
	IssueCodeTitle:                  "Title still contains 'unfoldingWord'",
	IssueCodeIdentifier:             "Identifier is not valid for the resource's subject",
	IssueCodeLanguage:               "Language is still English",
	IssueCodeIngredientTitle:        "Project title is still in English",
	IssueCodeIngredientMissing:      "Project file is missing",
	IssueCodeIngredientEmpty:        "Project file is empty",
	IssueCodeReleaseNeeded:          "An error-free release needs to be published for the resource",
	IssueCodeRelationMissing:        "Required relations are missing",
	IssueCodeOrigLangVersionMissing: "Relations for original languages are missing version",
	IssueCodeOBSStoryMissing:        "Not all 50 stories are present",
	IssueCodeOBSStoryTitleMissing:   "Stories are missing their titles",
	IssueCodeOBSWrongFrameCount:     "Stories are missing their frames",
	IssueCodeOBSBibleRefenceMissing: "Stories are missing their Bible references",
	IssueCodeUSFMInvalid:            "USFM file is missing or invalid",
	IssueCodeUSFMNoAlignment:        "USFM file has no alignment data",
	IssueCodeSBIngredientMismatch:   "Ingredient entries do not match the files in the repo",
}

var IssueCodePositives = map[IssueCode]string{
	IssueCodeNoMetadata:             "Metadata found for the repository",
	IssueCodeMetadataInvalid:        "Valid Metadata",
	IssueCodeRelation:               "Relations use the language of the resource",
	IssueCodePublisher:              "Publisher has been properly changed",
	IssueCodeTitle:                  "Title has been properly changed",
	IssueCodeIdentifier:             "Identifier is valid for the resource's subject",
	IssueCodeLanguage:               "Language has been set",
	IssueCodeIngredientTitle:        "Project titles have been translated",
	IssueCodeIngredientMissing:      "Project files exist",
	IssueCodeIngredientEmpty:        "Project files are not empty",
	IssueCodeReleaseNeeded:          "An error-free release has been published",
	IssueCodeRelationMissing:        "Has all required relations",
	IssueCodeOrigLangVersionMissing: "Relations for original languages has version",
	IssueCodeOBSStoryMissing:        "All 50 stories are present",
	IssueCodeOBSStoryTitleMissing:   "All stories have titles",
	IssueCodeOBSWrongFrameCount:     "All stories have all their frames",
	IssueCodeOBSBibleRefenceMissing: "All stories have Bible references",
	IssueCodeUSFMInvalid:            "USFM file is valid",
	IssueCodeUSFMNoAlignment:        "USFM file has alignment data",
	IssueCodeSBIngredientMismatch:   "Ingredient entries match the files in the repo",
}

// IssueDetailsFormatStrings are the Details format strings per issue code. Codes that apply
// to more than one metadata type take the metadata file name (e.g. manifest.yaml,
// manifest.json, metadata.json) as their first format argument.
var IssueDetailsFormatStrings = map[IssueCode]string{
	IssueCodeNoMetadata:             "No metadata found for the repository. Is not a resource repository.",
	IssueCodeMetadataInvalid:        "Metadata is invalid. The file does not match the schema.",
	IssueCodeRelation:               "Relation in manifest.yaml **`%s`** does not match resource language **`%s`**.",
	IssueCodePublisher:              "Publisher in manifest.yaml is still 'unfoldingWord'.",
	IssueCodeTitle:                  "Resource title in the %s file still contains 'unfoldingWord'.",
	IssueCodeIdentifier:             "Identifier in manifest.yaml should not be **`%s`** for the subject **`%s`**.",
	IssueCodeLanguage:               "Language in the %s file is still English **`en`**.",
	IssueCodeIngredientTitle:        "The title for the project '**`%s`**' is still in English: **`%s`**",
	IssueCodeIngredientMissing:      "The file for project **`%s`** does not exist in the repo: **`%s`**",
	IssueCodeIngredientEmpty:        "The file for project **`%s`** is empty: **`%s`**",
	IssueCodeReleaseNeeded:          "An error-free release needs to be published for the resource.",
	IssueCodeRelationMissing:        "Relations missing in manifest.yaml file: **`%s`**",
	IssueCodeOrigLangVersionMissing: "The relation **`%s`** is missing a version in the manifest.yaml file",
	IssueCodeOBSStoryMissing:        "The following stories are missing: **`%s`**",
	IssueCodeOBSStoryTitleMissing:   "The following stories are missing titles: **`%s`**",
	IssueCodeOBSWrongFrameCount:     "The following stories have no frames: **`%s`**",
	IssueCodeOBSBibleRefenceMissing: "The following stories are missing Bible references: **`%s`**",
	IssueCodeUSFMInvalid:            "The USFM file **`%s`** %s.",
	IssueCodeUSFMNoAlignment:        "The USFM file **`%s`** does not contain alignment data.",
	IssueCodeSBIngredientMismatch:   "The ingredient **`%s`** %s.",
}

// IssueSuggestionsFormatStrings are the Suggestion format strings per issue code. Suggestions
// that tell the user to edit the metadata file take an HTML link to that file (see
// the door43healthcheck service's metadataFileLink) as their first format argument.
var IssueSuggestionsFormatStrings = map[IssueCode]string{
	IssueCodeNoMetadata:             "Add a manifest.yaml file to the repository to describe the resource.",
	IssueCodeMetadataInvalid:        "Edit the %s file and fix these errors:\n\n<pre>%s</pre>",
	IssueCodeRelation:               "Edit the %s file and change **`%s`** to **`%s/%s`** in the **`relation`** field.",
	IssueCodePublisher:              "Edit the %s file and change `unfoldingWord` to the correct publisher in the **`publisher`** field, such as **`%s`**.",
	IssueCodeTitle:                  "Edit the %s file and remove 'unfoldingWord ' from the beginning of **`title`**, such as **`%s`** => **`%s`**, or translate into your language.",
	IssueCodeIdentifier:             "Edit the %s file and change **`%s`** to the correct **`identifier`** for the subject **`%s`**, which is **`%s`**.",
	IssueCodeLanguage:               "Edit the %s file and change **`en`** to the correct **`language code`** for your project's language, the **`title`** of the language, and the **`direction`**.",
	IssueCodeIngredientTitle:        "Edit the %s file and translate the **`title`** of the projects. For example, translate **'%s'** to the resource's language.",
	IssueCodeIngredientMissing:      "Either edit the %s file and remove the project with the identifier of **`%s`** or add the missing file, **`%s`**, to the repository.",
	IssueCodeIngredientEmpty:        "Either edit the %s file and remove the project with the identifier of **`%s`** or add content to the file **`%s`**.",
	IssueCodeReleaseNeeded:          "It looks like %s of the **`%s`** branch's %ss has been fixed. You should create a release for the resource with <a href=\"https://gateway-admin.netlify.app/\" target=\"_blank\">**`gatewayAdmin`**</a>.",
	IssueCodeRelationMissing:        "Edit the %s file and add the following relations: **`%s`**",
	IssueCodeOrigLangVersionMissing: "Edit the %s file and add version used for the relation **`%s`**.",
	IssueCodeOBSStoryMissing:        "Add and translate the following stories: **`%s`**.",
	IssueCodeOBSStoryTitleMissing:   "Add titles to the following stories: **`%s`**. Translate the titles from the English version.",
	IssueCodeOBSWrongFrameCount:     "Add frames to the following stories: **`%s`**. Check the English version for the expected frames.",
	IssueCodeOBSBibleRefenceMissing: "The following stories are missing Bible references: **`%s`**. Find the needed Bible references at the end of the English stories.",
	IssueCodeUSFMInvalid:            "Upload a valid USFM file for the book **`%s`** at **`%s`**.",
	IssueCodeUSFMNoAlignment:        "Align the book with translationCore or a compatible tool so the USFM file contains alignment data.",
	IssueCodeSBIngredientMismatch:   "Update the %s file so the **`ingredients`** entries match the files in the repository, or update the files themselves.",
}

// IssuePositiveString returns the summary format string for the issue in positive form
func (id IssueCode) IssuePositiveString() string {
	return IssueCodePositives[id]
}

// IssueNegativeString returns the summary format string for the issue in negative form
func (id IssueCode) IssueNegativeString() string {
	return IssueCodeNegatives[id]
}

// IssueDetailsFormatString returns the details format string for the issue
func (id IssueCode) IssueDetailsFormatString() string {
	return IssueDetailsFormatStrings[id]
}

// IssueSuggestionFormatString returns the suggestion format string for the issue
func (id IssueCode) IssueSuggestionFormatString() string {
	return IssueSuggestionsFormatStrings[id]
}

// Door43HealthcheckIssue represents a single health check issue for a resource.
// The stored rows for a Door43Metadata entry are replaced each time its health check runs.
type Door43HealthcheckIssue struct {
	ID            int64         `xorm:"pk autoincr" json:"-"`
	DMID          int64         `xorm:"'dm_id' INDEX NOT NULL" json:"-"`
	RepoID        int64         `xorm:"INDEX NOT NULL" json:"-"`
	IssueCode     IssueCode     `xorm:"NOT NULL" json:"issue_code"`
	SeverityLevel SeverityLevel `xorm:"NOT NULL" json:"severity_level"`
	PositiveTitle string        `json:"positive_title"`
	NegativeTitle string        `json:"negative_title"`
	Details       string        `xorm:"TEXT" json:"details"`
	Suggestion    string        `xorm:"TEXT" json:"suggestion"`
}

func init() {
	db.RegisterModel(new(Door43HealthcheckIssue))
}

func (h *Door43HealthcheckIssue) TableName() string {
	return "door43_healthcheck_issue"
}

// ReplaceDoor43HealthcheckIssues replaces all stored health check issues for a Door43Metadata entry
func ReplaceDoor43HealthcheckIssues(ctx context.Context, dm *Door43Metadata, issues []*Door43HealthcheckIssue) error {
	if dm.ID == 0 {
		return nil
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("dm_id = ?", dm.ID).Delete(new(Door43HealthcheckIssue)); err != nil {
			return err
		}
		if len(issues) == 0 {
			return nil
		}
		for _, issue := range issues {
			issue.ID = 0
			issue.DMID = dm.ID
			issue.RepoID = dm.RepoID
		}
		return db.Insert(ctx, issues)
	})
}

// GetDoor43HealthcheckIssuesByDMID returns the stored issues for a Door43Metadata entry in check order
func GetDoor43HealthcheckIssuesByDMID(ctx context.Context, dmID int64) ([]*Door43HealthcheckIssue, error) {
	issues := []*Door43HealthcheckIssue{}
	return issues, db.GetEngine(ctx).Where("dm_id = ?", dmID).OrderBy("id").Find(&issues)
}

// DeleteDoor43HealthcheckIssuesByDMID deletes the stored issues for a Door43Metadata entry
func DeleteDoor43HealthcheckIssuesByDMID(ctx context.Context, dmID int64) error {
	_, err := db.GetEngine(ctx).Where("dm_id = ?", dmID).Delete(new(Door43HealthcheckIssue))
	return err
}

// DeleteDoor43HealthcheckIssuesByRepoID deletes all stored issues for a repo
func DeleteDoor43HealthcheckIssuesByRepoID(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Delete(new(Door43HealthcheckIssue))
	return err
}

// HealthcheckFunc is a function variable that holds the healthcheck implementation.
// It is set by the service layer to avoid circular imports between models and services.
// See services/door43healthcheck/healthcheck.go for the implementation.
var HealthcheckFunc func(ctx context.Context, dm *Door43Metadata) *HealthcheckGroupedIssues

// LoadHealthcheck loads the stored health check results for this entry
func (dm *Door43Metadata) LoadHealthcheck(ctx context.Context) (*HealthcheckGroupedIssues, error) {
	issues, err := GetDoor43HealthcheckIssuesByDMID(ctx, dm.ID)
	if err != nil {
		return nil, err
	}
	return NewHealthcheckGroupedIssues(dm.MetadataType, dm.Subject, issues), nil
}

// GetHealthcheck returns the stored health check results for this entry. When the entry has
// never been checked, or its stored issues no longer add up to its stored severity (rows
// written before issues were persisted), the check is re-run and stored via HealthcheckFunc.
// This allows templates to call dm.GetHealthcheck(ctx) without the model
// needing to import the service package.
func (dm *Door43Metadata) GetHealthcheck(ctx context.Context) *HealthcheckGroupedIssues {
	if dm.ID > 0 && dm.HealthcheckSeverity > 0 {
		if hgi, err := dm.LoadHealthcheck(ctx); err == nil && hgi.OverallSeverityLevel == dm.HealthcheckSeverity {
			return hgi
		}
	}
	if HealthcheckFunc != nil {
		return HealthcheckFunc(ctx, dm)
	}
	return nil
}

// RemoveStringFromSlice removes all occurrences of s from the slice
func RemoveStringFromSlice(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
