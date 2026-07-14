// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"

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
)

// HealthcheckGroupedIssues groups health check issues by issue code
type HealthcheckGroupedIssues struct {
	Issues               map[IssueCode][]*Door43HealthcheckIssue `json:"issues"`
	OverallSeverityLevel SeverityLevel                           `json:"overall_severity_level"`
	SeverityLevelCount   SeverityLevelCount                      `json:"severity_level_count"`
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

// NewHealthcheckGroupedIssues creates a new HealthcheckGroupedIssues for a given subject.
// Only issue codes applicable to the subject are initialized.
func NewHealthcheckGroupedIssues(subject string, issues []*Door43HealthcheckIssue) *HealthcheckGroupedIssues {
	hgi := &HealthcheckGroupedIssues{
		Issues:               make(map[IssueCode][]*Door43HealthcheckIssue),
		OverallSeverityLevel: SeverityLevelSuccess,
		SeverityLevelCount:   make(map[SeverityLevel]int),
		Subject:              subject,
	}
	for _, code := range IssueCodesForSubject(subject) {
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

// GetOrder returns the applicable issue codes for this healthcheck's subject
func (hgi *HealthcheckGroupedIssues) GetOrder() []IssueCode {
	return IssueCodesForSubject(hgi.Subject)
}

// Issue code lists by subject applicability

// commonIssueCodes are checked for all RC subjects
var commonIssueCodes = []IssueCode{
	IssueCodeNoMetadata,
	IssueCodeMetadataInvalid,
	IssueCodeRelation,
	IssueCodePublisher,
	IssueCodeTitle,
	IssueCodeIdentifier,
	IssueCodeLanguage,
	IssueCodeIngredientTitle,
	IssueCodeIngredientMissing,
	IssueCodeIngredientEmpty,
	IssueCodeReleaseNeeded,
	IssueCodeRelationMissing,
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

// IssueCodesForSubject returns the issue codes applicable to the given subject
func IssueCodesForSubject(subject string) []IssueCode {
	codes := append([]IssueCode{}, commonIssueCodes...)
	switch subject {
	case "Open Bible Stories":
		codes = append(codes, obsIssueCodes...)
	case "TSV Translation Notes":
		codes = append(codes, tnIssueCodes...)
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
}

var IssueDetailsFormatStrings = map[IssueCode]string{
	IssueCodeNoMetadata:             "No metadata found for the repository. Is not a resource repository.",
	IssueCodeMetadataInvalid:        "Metadata is invalid. The file does not match the schema.",
	IssueCodeRelation:               "Relation in manifest.yaml **`%s`** does not match resource language **`%s`**.",
	IssueCodePublisher:              "Publisher in manifest.yaml is still 'unfoldingWord'.",
	IssueCodeTitle:                  "Resource title in manifest.yaml still contains 'unfoldingWord'.",
	IssueCodeIdentifier:             "Identifier in manifest.yaml should not be **`%s`** for the subject **`%s`**.",
	IssueCodeLanguage:               "Language in manifest.yaml is still English **`en`**.",
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
}

var IssueSuggestionsFormatStrings = map[IssueCode]string{
	IssueCodeNoMetadata:             "Add a manifest.yaml file to the repository to describe the resource.",
	IssueCodeMetadataInvalid:        "Edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and fix these errors:\n\n<pre>%s</pre>",
	IssueCodeRelation:               "Edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and change **`%s`** to **`%s/%s`** in the **`relation`** field.",
	IssueCodePublisher:              "Edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and change `unfoldingWord` to the correct publisher in the **`publisher`** field, such as **`%s`**.",
	IssueCodeTitle:                  "Edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and remove 'unfoldingWord ' from the beginning of **`title`**, such as **`%s`** => **`%s`**, or translate into your language.",
	IssueCodeIdentifier:             "Edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and change **`%s`** to the correct **`identifier`** for the subject **`%s`**, which is **`%s`**.",
	IssueCodeLanguage:               "Edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and change **`en`** to the correct **`language code`** for your project's language, the **`title`** of the language, and the **`direction`**.",
	IssueCodeIngredientTitle:        "Edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and translate the **`title`** of the projects. For example, translate **'%s'** to the resource's language.",
	IssueCodeIngredientMissing:      "Either edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and remove the project with the identifier of **`%s`** or add the missing file, **`%s`**, to the repository.",
	IssueCodeIngredientEmpty:        "Either edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and remove the project with the identifier of **`%s`** or add content to the file **`%s`**.",
	IssueCodeReleaseNeeded:          "It looks like %s of the **`%s`** branch's %ss has been fixed. You should create a release for the resource with <a href=\"https://gateway-admin.netlify.app/\" target=\"_blank\">**`gatewayAdmin`**</a>.",
	IssueCodeRelationMissing:        "Edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and add the following relations: **`%s`**",
	IssueCodeOrigLangVersionMissing: "Edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and add version used for the relation **`%s`**.",
	IssueCodeOBSStoryMissing:        "Add and translate the following stories: **`%s`**.",
	IssueCodeOBSStoryTitleMissing:   "Add titles to the following stories: **`%s`**. Translate the titles from the English version.",
	IssueCodeOBSWrongFrameCount:     "Add frames to the following stories: **`%s`**. Check the English version for the expected frames.",
	IssueCodeOBSBibleRefenceMissing: "The following stories are missing Bible references: **`%s`**. Find the needed Bible references at the end of the English stories.",
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

// Door43HealthcheckIssue represents a single health check issue for a resource
type Door43HealthcheckIssue struct {
	IssueCode     IssueCode     `json:"issue_code"`
	SeverityLevel SeverityLevel `json:"severity_level"`
	PositiveTitle string        `json:"positive_title"`
	NegativeTitle string        `json:"negative_title"`
	Details       string        `json:"details"`
	Suggestion    string        `json:"suggestion"`
}

func (h *Door43HealthcheckIssue) TableName() string {
	return "door43_healthcheck_issue"
}

// HealthcheckFunc is a function variable that holds the healthcheck implementation.
// It is set by the service layer to avoid circular imports between models and services.
// See services/door43healthcheck/healthcheck.go for the implementation.
var HealthcheckFunc func(ctx context.Context, dm *Door43Metadata) *HealthcheckGroupedIssues

// GetHealthcheck delegates to the registered healthcheck service function.
// This allows templates to call dm.GetHealthcheck(ctx) without the model
// needing to import the service package.
func (dm *Door43Metadata) GetHealthcheck(ctx context.Context) *HealthcheckGroupedIssues {
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
