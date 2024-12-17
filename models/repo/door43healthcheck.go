// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/builder"
)

// SeverityLevel represents the level of severity or concern for a health check
type SeverityLevel int

const (
	SeverityLevelError SeverityLevel = iota
	SeverityLevelWarning
	SeverityLevelInfo
	SeverityLevelSuccess
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

// String returns string repensation of a SeverityLevel (int)
func (sl SeverityLevel) String() string {
	return SeverityLevelToStringMap[sl]
}

// IssueCode represents a specific issue type for a health check
type IssueCode string

// IssueCode values
const (
	IssueCodeNoMetadata        IssueCode = "no_metadata"            // no metadata found for the repository
	IssueCodeMetadataInvalid   IssueCode = "metadata_invalid"       // metadata is invalid
	IssueCodeRelation          IssueCode = "relation_lang_invalid"  // relation not valid
	IssueCodePublisher         IssueCode = "publisher_has_uw"       // publisher empty or is 'unfoldingWord'
	IssueCodeTitle             IssueCode = "title_has_uw"           // title is empty or still contains 'unfoldingWord'
	IssueCodeIdentifier        IssueCode = "abbreviation_invalid"   // abbreviation is not valid for given subject
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
	IssueCodeIdentifier,
	IssueCodeLanguage,
	IssueCodeIngredientTitle,
	IssueCodeIngredientMissing,
	IssueCodeIngredientEmpty,
	IssueCodeReleaseNeeded,
}

type HealthcheckGroupedIssues struct {
	Issues               map[IssueCode][]*Door43HealthcheckIssue
	OverallSeverityLevel SeverityLevel
	SeverityLevelCount   map[SeverityLevel]int
}

func NewHealthcheckGroupedIssues(issues []*Door43HealthcheckIssue) *HealthcheckGroupedIssues {
	hgi := &HealthcheckGroupedIssues{
		Issues:               make(map[IssueCode][]*Door43HealthcheckIssue),
		OverallSeverityLevel: SeverityLevelSuccess,
		SeverityLevelCount:   make(map[SeverityLevel]int),
	}
	for _, code := range IssueCodeOrder {
		hgi.Issues[code] = []*Door43HealthcheckIssue{}
	}
	for _, issue := range issues {
		hgi.AddIssue(issue)
	}
	return hgi
}

func (hgi *HealthcheckGroupedIssues) AddIssue(issue *Door43HealthcheckIssue) {
	hgi.Issues[issue.IssueCode] = append(hgi.Issues[issue.IssueCode], issue)
	if issue.SeverityLevel < hgi.OverallSeverityLevel {
		hgi.OverallSeverityLevel = issue.SeverityLevel
	}
	hgi.SeverityLevelCount[issue.SeverityLevel]++
}

func (hgi *HealthcheckGroupedIssues) GetIssues(issueCode IssueCode) []*Door43HealthcheckIssue {
	return hgi.Issues[issueCode]
}

func (hgi *HealthcheckGroupedIssues) GetOrder() []IssueCode {
	return IssueCodeOrder
}

var IssueCodeNegatives = map[IssueCode]string{
	IssueCodeNoMetadata:        "No metadata found for the repository",
	IssueCodeMetadataInvalid:   "Invalid metadata; file does not match schema",
	IssueCodeRelation:          "Relation is not the language of the resource",
	IssueCodePublisher:         "Publisher is still 'unfoldingWord'",
	IssueCodeTitle:             "Title still contains 'unfoldingWord'",
	IssueCodeIdentifier:        "Identifier is not valid for the resource's subject",
	IssueCodeLanguage:          "Language is still English",
	IssueCodeIngredientTitle:   "Ingredient is still in English",
	IssueCodeIngredientMissing: "Ingredient path is missing",
	IssueCodeIngredientEmpty:   "Ingredient file is empty",
	IssueCodeReleaseNeeded:     "An error-free release needs to be published for the resource",
}

var IssueCodePositives = map[IssueCode]string{
	IssueCodeNoMetadata:        "Metadata found for the repository",
	IssueCodeMetadataInvalid:   "Valid Metadata",
	IssueCodeRelation:          "Relations use the language of the resource",
	IssueCodePublisher:         "Publisher has been properly changed",
	IssueCodeTitle:             "Title has been properly changed",
	IssueCodeIdentifier:        "Identifier is valid for the resource's subject",
	IssueCodeLanguage:          "Language has been set",
	IssueCodeIngredientTitle:   "Ingredient titles have been translated",
	IssueCodeIngredientMissing: "Ingredient paths exists",
	IssueCodeIngredientEmpty:   "Ingredient files are not empty",
	IssueCodeReleaseNeeded:     "An error-free release has been published",
}

var IssueDetailsFormatStrings = map[IssueCode]string{
	IssueCodeNoMetadata:        "No metadata found for the repository. Is not a resource repository.",
	IssueCodeMetadataInvalid:   "Metadata is invalid. The file does not match the schema.",
	IssueCodeRelation:          "Relation in manifest.yaml **`%s`** does not match resource language **`%s`**.",
	IssueCodePublisher:         "Publisher in manifest.yaml is still 'unfoldingWord'.",
	IssueCodeTitle:             "Resouce title in manifest.yaml still contains 'unfoldingWord'.",
	IssueCodeIdentifier:        "Identifier in manifest.yaml should not be **`%s`** for the subject **`%s`**.",
	IssueCodeLanguage:          "Language in manifest.yaml is still English **`en`**.",
	IssueCodeIngredientTitle:   "The title in for the project '%s' is still in English: %s",
	IssueCodeIngredientMissing: "The path for project **`%s`** is does not exist in the repo: **`%s`**",
	IssueCodeIngredientEmpty:   "The file for project **`%s`** is empty: **`%s`**",
	IssueCodeReleaseNeeded:     "An error-free release needs to be published for the resource.",
}

var IssueSuggestionsFormatStrings = map[IssueCode]string{
	IssueCodeNoMetadata:        "Add a manifest.yaml file to the repository to describe the resource.",
	IssueCodeMetadataInvalid:   "Edit the [manifest.yaml](%s/src/branch/%s/manifest.yaml) file and fix these errors:\n\n<pre>%s</pre>",
	IssueCodeRelation:          "Edit the [manifest.yaml](%s/src/branch/%s/manifest.yaml) file and change **`%s`** to **`%s/%s`** in the **`relation`** field.",
	IssueCodePublisher:         "Edit the [manifest.yaml](%s/src/branch/%s/manifest.yaml) file and change `unfoldingWord` to the correct publisher in the **`publisher`** field, such as **`%s`**.",
	IssueCodeTitle:             "Edit the [manifest.yaml](%s/src/branch/%s/manifest.yaml) file and remove 'unfoldingWord ' from the beginning of **`title`**, such as **`%s`** => **`%s`**, or translate into your language.",
	IssueCodeIdentifier:        "Edit the [manifest.yaml](%s/src/branch/%s/manifest.yaml) file and change **`%s`** to the correct **`identifier`** for the subject **`%s`**, which is **`%s`**.",
	IssueCodeLanguage:          "Edit the [manifest.yaml](%s/src/branch/%s/manifest.yaml) file and change **`en`** to the correct **`language code`** for your project's language, the **`title`** of the language, and the **`direction`**.",
	IssueCodeIngredientTitle:   "Edit the [manifest.yaml](%s/src/branch/%s/manifest.yaml) file and translate the **`title`** of the projects. For example, translate **'%s'** to the resource's language.",
	IssueCodeIngredientMissing: "Either edit the [manifest.yaml](%s/src/branch/%s/manifest.yaml) file and remove the project **`%s`** or add the missing file or path, **`%s`**, to the repository.",
	IssueCodeIngredientEmpty:   "Either edit the [manifest.yaml](%s/src/branch/%s/manifest.yaml) file and remove the project **`%s`** or add content to the file **`%s`**.",
	IssueCodeReleaseNeeded:     "It looks like %s of the **`%s`** branch's %ss has been fixed. You should create a release for the resource with [gatewayAdmin](https://gateway-admin.netlify.app/).",
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
	ID               int64 `xorm:"pk autoincr"`
	Door43MetadataID int64 `xorm:"INDEX"`
	IssueCode        IssueCode
	SeverityLevel    SeverityLevel
	Details          string
	Suggestion       string             `xorm:"MEDIUMTEXT"`
	Current          bool               `xorm:"default true"`
	CreatedUnix      timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(Door43HealthcheckIssue))
}

func (h *Door43HealthcheckIssue) TableName() string {
	return "door43_healthcheck_issue"
}

// GetDoor43HealthcheckIssuesByDoor43MetadataID returns all health check issues for a resource
func GetDoor43HealthcheckIssuesByDoor43MetadataID(ctx context.Context, dmID int64) ([]*Door43HealthcheckIssue, error) {
	var issues []*Door43HealthcheckIssue

	if dmID == 0 {
		return issues, nil
	}

	err := db.GetEngine(ctx).
		Where(builder.Eq{"door43_metadata_id": dmID}).
		And(builder.Eq{"current": true}).
		OrderBy("severity_level").
		Find(&issues)
	if err != nil {
		log.Error("Error getting health check issues: %v", err)
	}
	return issues, err
}

func GetHealthcheckGroupedIssues(ctx context.Context, dmID int64) (*HealthcheckGroupedIssues, error) {
	issues, err := GetDoor43HealthcheckIssuesByDoor43MetadataID(ctx, dmID)
	if err != nil {
		log.Error("Error getting health check issues: %v", err)
		return nil, err
	}
	healthcheck := NewHealthcheckGroupedIssues(issues)
	return healthcheck, nil
}

// PerformHealthcheck on Door43Metadata
func PerformHealthcheck(ctx context.Context, dm *Door43Metadata) (*HealthcheckGroupedIssues, error) {
	dm.LoadRepo(ctx)
	if dm.HealthcheckUnix > 0 {
		oldIssues, err := GetDoor43HealthcheckIssuesByDoor43MetadataID(ctx, dm.ID)
		if err != nil {
			return nil, err
		}
		for _, issue := range oldIssues {
			issue.Current = false
			if _, err := db.GetEngine(ctx).ID(issue.ID).Cols("current").Update(issue); err != nil {
				return nil, err
			}
		}
	}
	issues := []*Door43HealthcheckIssue{}

	// Check if metadata is valid
	if dm.ValidationError != nil {
		item := &Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        IssueCodeMetadataInvalid,
			SeverityLevel:    SeverityLevelError,
			Details:          IssueCodeMetadataInvalid.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(IssueCodeMetadataInvalid.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dcs.ConvertValidationErrorToString(dm.ValidationError)),
			Current:          true,
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return nil, err
		}
		issues = append(issues, item)
	}

	if dm.Repo.Owner.LowerName != "unfoldingword" && (dm.Publisher == "" || strings.HasPrefix(strings.TrimSpace(dm.Publisher), "unfoldingWord")) {
		item := &Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        IssueCodePublisher,
			SeverityLevel:    SeverityLevelError,
			Details:          IssueCodePublisher.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(IssueCodePublisher.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Repo.OwnerName),
			Current:          true,
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return nil, err
		}
		issues = append(issues, item)
	}

	if dm.Repo.Owner.LowerName != "unfoldingword" && (dm.Title == "" || strings.HasPrefix(strings.TrimSpace(dm.Title), "unfoldingWord")) {
		item := &Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        IssueCodeTitle,
			SeverityLevel:    SeverityLevelError,
			Details:          IssueCodeTitle.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(IssueCodeTitle.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Title, strings.Join(strings.Fields(dm.Title)[1:], " ")),
			Current:          true,
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return nil, err
		}
		issues = append(issues, item)
	}

	if dm.Abbreviation == "" || dm.Subject != "Bible" && dm.Subject != "Aligned Bible" {
		if subject, ok := dcs.ResourceToSubjectMap[dm.Abbreviation]; !ok || subject != dm.Subject {
			item := &Door43HealthcheckIssue{
				Door43MetadataID: dm.ID,
				IssueCode:        IssueCodeIdentifier,
				SeverityLevel:    SeverityLevelError,
				Details:          fmt.Sprintf(IssueCodeIdentifier.IssueDetailsFormatString(), dm.Abbreviation, dm.Subject),
				Suggestion:       fmt.Sprintf(IssueCodeIdentifier.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Abbreviation, dm.Subject, dcs.SubjectToResourceMap[dm.Subject]),
				Current:          true,
			}
			if _, err := db.GetEngine(ctx).Insert(item); err != nil {
				return nil, err
			}
			issues = append(issues, item)
		}
	}

	if dm.Language == "en" && !strings.HasPrefix(dm.Repo.Name, "en_") {
		item := &Door43HealthcheckIssue{
			Door43MetadataID: dm.ID,
			IssueCode:        IssueCodeLanguage,
			SeverityLevel:    SeverityLevelWarning,
			Details:          IssueCodeLanguage.IssueDetailsFormatString(),
			Suggestion:       fmt.Sprintf(IssueCodeLanguage.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref),
			Current:          true,
		}
		if _, err := db.GetEngine(ctx).Insert(item); err != nil {
			return nil, err
		}
		issues = append(issues, item)
	}

	// Check for relations in other languages
	for _, relation := range dm.Relations {
		if relation.Language != dm.Language && relation.Language != "hbo" && relation.Language != "el-x-koine" {
			item := &Door43HealthcheckIssue{
				Door43MetadataID: dm.ID,
				IssueCode:        IssueCodeRelation,
				SeverityLevel:    SeverityLevelError,
				Details:          fmt.Sprintf(IssueCodeRelation.IssueDetailsFormatString(), relation.FullRelation, dm.Language),
				Suggestion:       fmt.Sprintf(IssueCodeRelation.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, relation.FullRelation, dm.Language, relation.Identifier),
				Current:          true,
			}
			if _, err := db.GetEngine(ctx).Insert(item); err != nil {
				return nil, err
			}
			issues = append(issues, item)
		}
	}

	if dm.Ingredients != nil {
		doneIngredientTitle := false
		for _, ingredient := range dm.Ingredients {
			// Acts, Numbers and Deuteronomy are only in English and not other languages, so using those
			if !doneIngredientTitle && (ingredient.Title == "" || (dm.Language != "en" && (ingredient.Title == "Numbers" || ingredient.Title == "Deuteronomy" || ingredient.Title == "Acts"))) {
				doneIngredientTitle = true
				item := &Door43HealthcheckIssue{
					Door43MetadataID: dm.ID,
					IssueCode:        IssueCodeIngredientTitle,
					SeverityLevel:    SeverityLevelError,
					Details:          fmt.Sprintf(IssueCodeIngredientTitle.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Title),
					Suggestion:       fmt.Sprintf(IssueCodeIngredientTitle.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Title),
					Current:          true,
				}
				if _, err := db.GetEngine(ctx).Insert(item); err != nil {
					return nil, err
				}
				issues = append(issues, item)
			}

			if !ingredient.Exists {
				item := &Door43HealthcheckIssue{
					Door43MetadataID: dm.ID,
					IssueCode:        IssueCodeIngredientMissing,
					SeverityLevel:    SeverityLevelError,
					Details:          fmt.Sprintf(IssueCodeIngredientMissing.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Path),
					Suggestion:       fmt.Sprintf(IssueCodeIngredientMissing.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Identifier, ingredient.Path),
					Current:          true,
				}
				if _, err := db.GetEngine(ctx).Insert(item); err != nil {
					return nil, err
				}
				issues = append(issues, item)
			}

			if ingredient.Exists && ingredient.Size == 0 && !ingredient.IsDir {
				item := &Door43HealthcheckIssue{
					Door43MetadataID: dm.ID,
					IssueCode:        IssueCodeIngredientEmpty,
					SeverityLevel:    SeverityLevelError,
					Details:          fmt.Sprintf(IssueCodeIngredientEmpty.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Path),
					Suggestion:       fmt.Sprintf(IssueCodeIngredientEmpty.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Identifier, ingredient.Path),
					Current:          true,
				}
				if _, err := db.GetEngine(ctx).Insert(item); err != nil {
					return nil, err
				}
				issues = append(issues, item)
			}
		}
	}

	healthcheckGroupedIssues := NewHealthcheckGroupedIssues(issues)
	fmt.Printf("dm.Ref: %v\n", dm.Ref)
	fmt.Printf("dm.Repo.DefaultBranch: %v\n", dm.Repo.DefaultBranch)
	fmt.Printf("healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelError]: %v\n", healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelError])
	fmt.Printf("healthcheckGroupedIssues.SeverityLevelCount: %v\n", healthcheckGroupedIssues.SeverityLevelCount)
	if dm.Ref == dm.Repo.DefaultBranch {
		if healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelError] > 0 {
			item := &Door43HealthcheckIssue{
				Door43MetadataID: dm.ID,
				IssueCode:        IssueCodeReleaseNeeded,
				SeverityLevel:    SeverityLevelError,
				Details:          IssueCodeReleaseNeeded.IssueDetailsFormatString(),
				Suggestion:       "Fix all the errors above. Then make a release with [gatewayAdmin](https://gateway-admin.netlify.app/).",
				Current:          true,
			}
			if _, err := db.GetEngine(ctx).Insert(item); err != nil {
				return nil, err
			}
			healthcheckGroupedIssues.AddIssue(item)
		} else {
			dm.LoadRepo(ctx)
			err := dm.Repo.LoadLatestDMs(ctx)
			if err != nil {
				return nil, err
			}
			var releaseHealthcheckGroupedIssues *HealthcheckGroupedIssues
			if dm.Repo.LatestProdDM != nil {
				if dm.Repo.LatestProdDM.HealthcheckUnix == 0 {
					var err error
					if releaseHealthcheckGroupedIssues, err = PerformHealthcheck(ctx, dm.Repo.LatestProdDM); err != nil {
						return nil, err
					}
				} else {
					releaseHealthcheckGroupedIssues, err = GetHealthcheckGroupedIssues(ctx, dm.Repo.LatestProdDM.ID)
					if err != nil {
						return nil, err
					}
				}
			}

			if healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelError] == 0 && releaseHealthcheckGroupedIssues != nil && releaseHealthcheckGroupedIssues.SeverityLevelCount[SeverityLevelError] > 0 {
				item := &Door43HealthcheckIssue{
					Door43MetadataID: dm.ID,
					IssueCode:        IssueCodeReleaseNeeded,
					SeverityLevel:    SeverityLevelError,
					Details:          IssueCodeReleaseNeeded.IssueDetailsFormatString(),
					Suggestion:       fmt.Sprintf(IssueCodeReleaseNeeded.IssueSuggestionFormatString(), "all", dm.Ref, SeverityLevelError.String()),
					Current:          true,
				}
				if _, err := db.GetEngine(ctx).Insert(item); err != nil {
					return nil, err
				}
				healthcheckGroupedIssues.AddIssue(item)
			} else if releaseHealthcheckGroupedIssues != nil && healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelWarning] < releaseHealthcheckGroupedIssues.SeverityLevelCount[SeverityLevelWarning] {
				item := &Door43HealthcheckIssue{
					Door43MetadataID: dm.ID,
					IssueCode:        IssueCodeReleaseNeeded,
					SeverityLevel:    SeverityLevelInfo,
					Details:          IssueCodeReleaseNeeded.IssueDetailsFormatString(),
					Suggestion:       fmt.Sprintf(IssueCodeReleaseNeeded.IssueSuggestionFormatString(), "some or all", dm.Ref, SeverityLevelWarning.String()),
					Current:          true,
				}
				if _, err := db.GetEngine(ctx).Insert(item); err != nil {
					return nil, err
				}
				healthcheckGroupedIssues.AddIssue(item)
			}
		}
	}

	dm.HealthcheckUnix = timeutil.TimeStampNow()
	if _, err := db.GetEngine(ctx).ID(dm.ID).Cols("healthcheck_unix").Update(dm); err != nil {
		return nil, err
	}

	return healthcheckGroupedIssues, nil
}
