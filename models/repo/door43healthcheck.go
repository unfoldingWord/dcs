// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/builder"
)

// SeverityLevel represents the level of severity or concern for a health check
type SeverityLevel string

const (
	SeverityLevelError   SeverityLevel = "error"
	SeverityLevelWarning SeverityLevel = "warning"
	SeverityLevelInfo    SeverityLevel = "info"
	SeverityLevelSuccess SeverityLevel = "success"
)

type IssueCode string

// IssueCode values
const (
	IssueCodeNoMetadata        IssueCode = "no_metadata"            // no metadata found for the repository
	IssueCodeMetadataInvalid   IssueCode = "metadata_invalid"       // metadata is invalid
	IssueCodeRelation          IssueCode = "relation_lang_invalid"  // relation not valid
	IssueCodePublisher         IssueCode = "publisher_has_uw"       // publisher empty or is 'unfoldingWord'
	IssueCodeTitle             IssueCode = "title_has_uw"           // title is empty or still contains 'unfoldingWord'
	IssueCodeAbbreviation      IssueCode = "abbreviation_invalid"   // abbreviation is not valid for given subject
	IssueCodeLanguage          IssueCode = "language_is_en"         // language is empty or still English
	IssueCodeIngredientTitle   IssueCode = "ingredient_title_is_en" // ingredient title is empty or still English
	IssueCodeIngredientMissing IssueCode = "ingredient_missing"     // ingredient's path is missing
	IssueCodeIngredientEmpty   IssueCode = "ingredient_empty"       // ingredient's file is empty
	IssueCodeReleaseNeeded     IssueCode = "release_needed"         // a release is needed for the resource
)

var IssueCodeOrder = []IssueCode{
	IssueCodeNoMetadata,
	IssueCodeMetadataInvalid,
	IssueCodeRelation,
	IssueCodePublisher,
	IssueCodeTitle,
	IssueCodeAbbreviation,
	IssueCodeLanguage,
	IssueCodeIngredientTitle,
	IssueCodeIngredientMissing,
	IssueCodeIngredientEmpty,
	IssueCodeReleaseNeeded,
}

type OrderedIssuesMap struct {
	Issues map[IssueCode][]*Door43HealthcheckIssue
}

func NewOrderedIssuesMap() *OrderedIssuesMap {
	oim := &OrderedIssuesMap{
		Issues: make(map[IssueCode][]*Door43HealthcheckIssue),
	}
	for _, code := range IssueCodeOrder {
		oim.Issues[code] = []*Door43HealthcheckIssue{}
	}
	return oim
}

func (oim *OrderedIssuesMap) AddIssue(issue *Door43HealthcheckIssue) {
	oim.Issues[issue.IssueCode] = append(oim.Issues[issue.IssueCode], issue)
}

func (oim *OrderedIssuesMap) GetIssues(issueCode IssueCode) []*Door43HealthcheckIssue {
	return oim.Issues[issueCode]
}

func (oim *OrderedIssuesMap) GetOrder() []IssueCode {
	return IssueCodeOrder
}

var IssueCodeNegatives = map[IssueCode]string{
	IssueCodeNoMetadata:        "No metadata found for the repository",
	IssueCodeMetadataInvalid:   "Invalid metadata; file does not match schema",
	IssueCodeRelation:          "Relation is not the language of the resource",
	IssueCodePublisher:         "Publisher is still 'unfoldingWord'",
	IssueCodeTitle:             "Title still contains 'unfoldingWord'",
	IssueCodeAbbreviation:      "Abbreviation is not valid for given subject",
	IssueCodeLanguage:          "Language is still English",
	IssueCodeIngredientTitle:   "Ingredient is still in English",
	IssueCodeIngredientMissing: "Ingredient's path is missing",
	IssueCodeIngredientEmpty:   "Ingredient's file is empty",
	IssueCodeReleaseNeeded:     "A release is needed for the resource",
}

var IssueCodePositives = map[IssueCode]string{
	IssueCodeNoMetadata:        "Metadata found for the repository",
	IssueCodeMetadataInvalid:   "Valid Metadata",
	IssueCodeRelation:          "Relation is the language of the resource",
	IssueCodePublisher:         "Publisher has been properly changed",
	IssueCodeTitle:             "Title has been properly changed",
	IssueCodeAbbreviation:      "Abbreviation is valid for given subject",
	IssueCodeLanguage:          "Language has been set",
	IssueCodeIngredientTitle:   "Ingredients titles have been translated",
	IssueCodeIngredientMissing: "Ingredients paths exists",
	IssueCodeIngredientEmpty:   "Ingredients files are not empty",
	IssueCodeReleaseNeeded:     "A release has been created",
}

var IssueDetailsFormatStrings = map[IssueCode]string{
	IssueCodeNoMetadata:        "No metadata found for the repository. Is not a resource repository.",
	IssueCodeMetadataInvalid:   "Metadata is invalid. The file does not match the schema.",
	IssueCodeRelation:          "Relation in manifest.yaml **`%s`** does not match resource language **`%s`**.",
	IssueCodePublisher:         "Publisher in manifest.yaml is still 'unfoldingWord'.",
	IssueCodeTitle:             "Resouce title in manifest.yaml still contains 'unfoldingWord'.",
	IssueCodeAbbreviation:      "Abbreviation in manifest.yaml should not be **`%s`** for the subject **`%s`**.",
	IssueCodeLanguage:          "Language in manifest.yaml is still English (en).",
	IssueCodeIngredientTitle:   "The title in for the project '%s' is still in English: %s",
	IssueCodeIngredientMissing: "The path for project '%s' is does not exist: %s",
	IssueCodeIngredientEmpty:   "The file for project '%s' is empty: %s",
	IssueCodeReleaseNeeded:     "A release is needed for the resource.",
}

var IssueSuggestionsFormatStrings = map[IssueCode]string{
	IssueCodeNoMetadata:        "Add a manifest.yaml file to the repository to describe the resource.",
	IssueCodeMetadataInvalid:   "Edit the [manifest.yaml file](%s/src/branch/%s/manifest.yaml) and fix these errors:\n\n<pre>%s</pre>",
	IssueCodeRelation:          "Edit the [manifest.yaml file](%s/src/branch/%s/manifest.yaml) and change **`%s`** to **`%s/%s`**.",
	IssueCodePublisher:         "Edit the [manifest.yaml file](%s/src/branch/%s/manifest.yaml) and change `unfoldingWord` to the correct publisher, e.g. %s.",
	IssueCodeTitle:             "Edit the [manifest.yaml file](%s/src/branch/%s/manifest.yaml) and remove 'unfoldingWord ' from the beginning of title, or translate into your language.",
	IssueCodeAbbreviation:      "Edit the [manifest.yaml file](%s/src/branch/%s/manifest.yaml) and change `%s` to the correct abbreviation for the subject %s, which is %s.",
	IssueCodeLanguage:          "Edit the [manifest.yaml file](%s/src/branch/%s/manifest.yaml) and change `en` to the correct language code for your project's language, the title of the language, and the direction.",
	IssueCodeIngredientTitle:   "Edit the [manifest.yaml file](%s/src/branch/%s/manifest.yaml) and translate the title of the project from '%s' to the resource's language.",
	IssueCodeIngredientMissing: "Either edit the [manifest.yaml file](%s/src/branch/%s/manifest.yaml) remove the project '%s' or add the missing file or path to the repository.",
	IssueCodeIngredientEmpty:   "Either edit the [manifest.yaml file](%s/src/branch/%s/manifest.yaml) remove the project '%s' or add content to the file '%s'.",
	IssueCodeReleaseNeeded:     "Create a release for the resource with gatewayAdmin.",
}

// IssuePositiveString returns the summary format string for the issue in possitive form
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

// Door43HealtcheckIssue represents a single health check issue for a resource
type Door43HealthcheckIssue struct {
	ID               int64         `xorm:"pk autoincr"`
	Door43MetadataID int64         `xorm:"INDEX"`
	IssueCode        IssueCode     `xorm:"INDEX"`
	SeverityLevel    SeverityLevel `xorm:"INDEX"`
	Details          string
	Suggestion       string             `xorm:"MEDIUMTEXT"`
	CreatedUnix      timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(Door43HealthcheckIssue))
}

func (h *Door43HealthcheckIssue) TableName() string {
	return "door43_healthcheck_issue"
}

// HealthcheckGrouped is a class for full health check info
type HealthcheckGrouped struct {
}

// GetDoor43HealthcheckIssuesByDoor43MetadataID returns all health check issues for a resource
func GetDoor43HealthcheckIssuesByDoor43MetadataID(ctx context.Context, dmID int64) ([]*Door43HealthcheckIssue, error) {
	var issues []*Door43HealthcheckIssue

	if dmID == 0 {
		return issues, nil
	}

	err := db.GetEngine(ctx).
		Where(builder.Eq{"door43_metadata_id": dmID}).
		OrderBy("severity_level").
		Find(&issues)
	if err != nil {
		log.Error("Error getting health check issues: %v", err)
	}
	return issues, err
}

func GetOrderedHealthcheck(ctx context.Context, dmID int64) (*OrderedIssuesMap, error) {
	issues, err := GetDoor43HealthcheckIssuesByDoor43MetadataID(ctx, dmID)
	if err != nil {
		log.Error("Error getting health check issues: %v", err)
		return nil, err
	}
	healthcheck := NewOrderedIssuesMap()
	for _, issue := range issues {
		healthcheck.AddIssue(issue)
	}
	return healthcheck, nil
}
