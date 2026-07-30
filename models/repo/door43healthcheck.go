// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
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
	IssueCodeSBIngredientMissing    IssueCode = "sb_ingredient_missing"        // SB ingredient entry's file doesn't exist in the repo
	IssueCodeSBIngredientMismatch   IssueCode = "sb_ingredient_mismatch"       // SB ingredient entry's size/checksum doesn't match the file
	IssueCodeRepoNameLanguage       IssueCode = "repo_name_lang_mismatch"      // repo name's language prefix doesn't match the metadata language
	IssueCodeTSVHeaderInvalid       IssueCode = "tsv_header_invalid"           // TSV header row doesn't match the subject's column schema
	IssueCodeTSVRowInvalid          IssueCode = "tsv_row_invalid"              // TSV row has the wrong number of columns
	IssueCodeTSVIDInvalid           IssueCode = "tsv_id_invalid"               // TSV ID cell doesn't match ^[a-z][a-z0-9]{3}$
	IssueCodeTSVIDDuplicate         IssueCode = "tsv_id_duplicate"             // TSV ID is not unique within the file
	IssueCodeTSVReferenceInvalid    IssueCode = "tsv_reference_invalid"        // TSV Reference cell doesn't match the reference grammar
	IssueCodeTSVOccurrenceInvalid   IssueCode = "tsv_occurrence_invalid"       // TSV Occurrence cell is not an integer >= -1
	IssueCodeTSVLinkInvalid         IssueCode = "tsv_link_invalid"             // TSV TWLink/SupportReference doesn't match the rc:// grammar
	IssueCodeTSVCellEmpty           IssueCode = "tsv_cell_empty"               // TSV required content cell is empty
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
	if len(hgi.Issues[IssueCodeMetadataInvalid]) > 0 {
		// a schema-invalid metadata file fails L1, so no deeper check ran — showing
		// their codes would render misleading green checkmarks for unchecked rules
		return []IssueCode{IssueCodeMetadataInvalid}
	}
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
	IssueCodeRepoNameLanguage,
	IssueCodeRelation,
	IssueCodeRelationMissing,
}

// usfmIssueCodes are checked for scripture subjects of rc, sb and tc repos
var usfmIssueCodes = []IssueCode{
	IssueCodeUSFMInvalid,
	IssueCodeUSFMNoAlignment,
}

// tsvIssueCodes are checked for TSV subjects of rc and sb repos
var tsvIssueCodes = []IssueCode{
	IssueCodeTSVHeaderInvalid,
	IssueCodeTSVRowInvalid,
	IssueCodeTSVIDInvalid,
	IssueCodeTSVIDDuplicate,
	IssueCodeTSVReferenceInvalid,
	IssueCodeTSVOccurrenceInvalid,
	IssueCodeTSVLinkInvalid,
	IssueCodeTSVCellEmpty,
}

// sbIssueCodes are only checked for Scripture Burrito (sb) repos
var sbIssueCodes = []IssueCode{
	IssueCodeSBIngredientMissing,
	IssueCodeSBIngredientMismatch,
}

// IsScriptureSubject returns true for subjects whose content files are USFM books
func IsScriptureSubject(subject string) bool {
	switch subject {
	case "Bible", "Aligned Bible", "Greek New Testament", "Hebrew Old Testament":
		return true
	}
	return false
}

// IsTSVSubject returns true for subjects whose content files are TSV files
func IsTSVSubject(subject string) bool {
	return strings.HasPrefix(subject, "TSV ")
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
		switch {
		case subject == "Open Bible Stories":
			codes = append(codes, obsIssueCodes...)
		case IsScriptureSubject(subject):
			codes = append(codes, usfmIssueCodes...)
		case IsTSVSubject(subject):
			codes = append(codes, tsvIssueCodes...)
		}
		if subject == "TSV Translation Notes" {
			codes = append(codes, tnIssueCodes...)
		}
	case "tc":
		codes = append(codes, usfmIssueCodes...)
	case "sb":
		codes = append(codes, IssueCodeRepoNameLanguage)
		codes = append(codes, sbIssueCodes...)
		switch {
		case IsScriptureSubject(subject):
			codes = append(codes, usfmIssueCodes...)
		case IsTSVSubject(subject):
			codes = append(codes, tsvIssueCodes...)
		}
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
	IssueCodeUSFMInvalid:            "USFM files are missing or invalid",
	IssueCodeUSFMNoAlignment:        "USFM files have no alignment data",
	IssueCodeSBIngredientMissing:    "Ingredient files are missing from the repo",
	IssueCodeSBIngredientMismatch:   "Ingredient sizes or checksums do not match the files in the repo",
	IssueCodeRepoNameLanguage:       "Repo name's language does not match the metadata language",
	IssueCodeTSVHeaderInvalid:       "TSV files have an invalid header row",
	IssueCodeTSVRowInvalid:          "TSV rows have the wrong number of columns",
	IssueCodeTSVIDInvalid:           "TSV rows have invalid IDs",
	IssueCodeTSVIDDuplicate:         "TSV rows have duplicate IDs",
	IssueCodeTSVReferenceInvalid:    "TSV rows have invalid references",
	IssueCodeTSVOccurrenceInvalid:   "TSV rows have invalid occurrences",
	IssueCodeTSVLinkInvalid:         "TSV rows have invalid rc:// links",
	IssueCodeTSVCellEmpty:           "TSV rows are missing required content",
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
	IssueCodeUSFMInvalid:            "USFM files are valid",
	IssueCodeUSFMNoAlignment:        "USFM files have alignment data",
	IssueCodeSBIngredientMissing:    "Ingredient files all exist in the repo",
	IssueCodeSBIngredientMismatch:   "Ingredient sizes and checksums match the files in the repo",
	IssueCodeRepoNameLanguage:       "Repo name matches the metadata language",
	IssueCodeTSVHeaderInvalid:       "TSV files have valid header rows",
	IssueCodeTSVRowInvalid:          "TSV rows have the correct number of columns",
	IssueCodeTSVIDInvalid:           "TSV row IDs are valid",
	IssueCodeTSVIDDuplicate:         "TSV row IDs are unique",
	IssueCodeTSVReferenceInvalid:    "TSV references are valid",
	IssueCodeTSVOccurrenceInvalid:   "TSV occurrences are valid",
	IssueCodeTSVLinkInvalid:         "TSV rc:// links are well-formed",
	IssueCodeTSVCellEmpty:           "TSV rows have their required content",
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
	IssueCodeSBIngredientMissing:    "The ingredient **`%s`** does not exist in the repository.",
	IssueCodeSBIngredientMismatch:   "The ingredient **`%s`** %s.",
	IssueCodeRepoNameLanguage:       "The repo name **`%s`** indicates language **`%s`** but the metadata declares **`%s`**.",
	IssueCodeTSVHeaderInvalid:       "%s",
	IssueCodeTSVRowInvalid:          "%s",
	IssueCodeTSVIDInvalid:           "%s",
	IssueCodeTSVIDDuplicate:         "%s",
	IssueCodeTSVReferenceInvalid:    "%s",
	IssueCodeTSVOccurrenceInvalid:   "%s",
	IssueCodeTSVLinkInvalid:         "%s",
	IssueCodeTSVCellEmpty:           "%s",
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
	IssueCodeSBIngredientMissing:    "Update the %s file so the **`ingredients`** entries match the files in the repository, or add the missing files.",
	IssueCodeSBIngredientMismatch:   "Update the %s file so the **`ingredients`** entries match the files in the repository, or update the files themselves.",
	IssueCodeRepoNameLanguage:       "Rename the repository to start with **`%s_`** or fix the **`language`** in the %s file.",
	IssueCodeTSVHeaderInvalid:       "Fix the header row to exactly match the column schema for this resource type (see the details).",
	IssueCodeTSVRowInvalid:          "Fix the listed rows so every row has exactly as many tab-separated columns as the header; keep tabs and newlines out of cells (use a literal `\\n` for line breaks in notes).",
	IssueCodeTSVIDInvalid:           "Give the listed rows a 4-character ID matching **`^[a-z][a-z0-9]{3}$`**.",
	IssueCodeTSVIDDuplicate:         "Give every row a unique ID within its file.",
	IssueCodeTSVReferenceInvalid:    "Fix the listed Reference cells to use **`front:intro`**, **`{chapter}:intro`**, **`{chapter}:front`**, **`{chapter}:{verse}`**, **`{chapter}:{verse}-{verse}`** or verse lists like **`5:1,3,8-12`** (semicolon or comma separated; a bare verse or range continues the last named chapter).",
	IssueCodeTSVOccurrenceInvalid:   "Fix the listed Occurrence cells: an integer of -1 or greater when the Quote cell has text; 0 or blank when the Quote cell is empty (e.g. intro rows).",
	IssueCodeTSVLinkInvalid:         "Fix the listed links to match the rc:// grammar: **`rc://*/ta/man/{manual}/{slug}`** for TA links and **`rc://*/tw/dict/bible/{category}/{slug}`** for TW links.",
	IssueCodeTSVCellEmpty:           "Fill in the listed rows' required content cell (Note, Question or TWLink).",
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
// Rule, when set, is the rule ID from the DCS Resource Validation Specification
// (e.g. FILE-001); DCS-specific checks with no spec rule leave it empty.
type Door43HealthcheckIssue struct {
	ID            int64         `xorm:"pk autoincr" json:"-"`
	DMID          int64         `xorm:"'dm_id' INDEX NOT NULL" json:"-"`
	RepoID        int64         `xorm:"INDEX NOT NULL" json:"-"`
	IssueCode     IssueCode     `xorm:"NOT NULL" json:"issue_code"`
	Rule          string        `xorm:"VARCHAR(20)" json:"rule,omitempty"`
	SeverityLevel SeverityLevel `xorm:"NOT NULL" json:"severity_level"`
	PositiveTitle string        `json:"positive_title"`
	NegativeTitle string        `json:"negative_title"`
	Details       string        `xorm:"TEXT" json:"details"`
	Suggestion    string        `xorm:"TEXT" json:"suggestion"`
}

// IssueCodeDefaultRules maps issue codes to their rule ID in the DCS Resource Validation
// Specification. Codes that cover more than one rule get the rule set per finding by the
// check; codes not listed are DCS-specific with no spec rule.
var IssueCodeDefaultRules = map[IssueCode]string{
	IssueCodeMetadataInvalid:        "META-002",
	IssueCodeIngredientMissing:      "FILE-001",
	IssueCodeRelation:               "REL-004",
	IssueCodeRelationMissing:        "REL-001",
	IssueCodeRepoNameLanguage:       "META-019",
	IssueCodeSBIngredientMissing:    "FILE-001",
	IssueCodeSBIngredientMismatch:   "META-015",
	IssueCodeUSFMInvalid:            "USFM-001",
	IssueCodeUSFMNoAlignment:        "USFM-009",
	IssueCodeTSVHeaderInvalid:       "TSV-001",
	IssueCodeTSVRowInvalid:          "TSV-002",
	IssueCodeTSVIDInvalid:           "TSV-004",
	IssueCodeTSVIDDuplicate:         "TSV-005",
	IssueCodeTSVReferenceInvalid:    "TSV-003",
	IssueCodeTSVOccurrenceInvalid:   "TSV-006",
	IssueCodeTSVLinkInvalid:         "TSV-008",
	IssueCodeTSVCellEmpty:           "TSV-011",
	IssueCodeOBSStoryMissing:        "COMP-020",
	IssueCodeOBSStoryTitleMissing:   "MD-002",
	IssueCodeOBSWrongFrameCount:     "MD-002",
	IssueCodeOBSBibleRefenceMissing: "MD-002",
}

func init() {
	db.RegisterModel(new(Door43HealthcheckIssue))
}

func (h *Door43HealthcheckIssue) TableName() string {
	return "door43_healthcheck_issue"
}

// StoreHealthcheckResults stores the severity, counts, check time and issues for the
// entry in one transaction. It returns false without writing anything when the entry no
// longer exists — the ref was deleted while its check was running — so a mid-check
// branch/tag deletion never leaves orphaned results behind. On MySQL the existence check
// locks the row (SELECT ... FOR UPDATE) so a concurrent delete fully serializes against
// the write; on SQLite writers are serialized by the database itself.
func StoreHealthcheckResults(ctx context.Context, dm *Door43Metadata, issues []*Door43HealthcheckIssue) (bool, error) {
	if dm.ID == 0 {
		return false, nil
	}
	stored := false
	err := db.WithTx(ctx, func(ctx context.Context) error {
		sess := db.GetEngine(ctx).ID(dm.ID)
		if setting.Database.Type.IsMySQL() {
			sess = sess.ForUpdate()
		}
		found, err := sess.Exist(new(Door43Metadata))
		if err != nil {
			return err
		}
		if !found {
			return nil // the entry (its ref) was deleted while the check ran
		}
		if _, err := db.GetEngine(ctx).ID(dm.ID).Cols("healthcheck_severity", "healthcheck_counts", "healthcheck_time_unix").Update(dm); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Where("dm_id = ?", dm.ID).Delete(new(Door43HealthcheckIssue)); err != nil {
			return err
		}
		for _, issue := range issues {
			issue.ID = 0
			issue.DMID = dm.ID
			issue.RepoID = dm.RepoID
		}
		if len(issues) > 0 {
			if err := db.Insert(ctx, issues); err != nil {
				return err
			}
		}
		stored = true
		return nil
	})
	return stored, err
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
