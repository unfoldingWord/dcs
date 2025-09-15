// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
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

// String returns string repensation of a SeverityLevel (int)
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
	IssueCodeRelationMissing,
	IssueCodeOrigLangVersionMissing,
	IssueCodeOBSStoryMissing,
	IssueCodeOBSStoryTitleMissing,
	IssueCodeOBSWrongFrameCount,
	IssueCodeOBSBibleRefenceMissing,
}

type HealthcheckGroupedIssues struct {
	Issues               map[IssueCode][]*Door43HealthcheckIssue `json:"issues"`
	OverallSeverityLevel SeverityLevel                           `json:"overall_severity_level"`
	SeverityLevelCount   SeverityLevelCount                      `json:"severity_level_count"`
}

type SeverityLevelCount map[SeverityLevel]int

func (slc SeverityLevelCount) MarshalJSON() ([]byte, error) {
	result := make(map[string]int)
	for level, count := range slc {
		result[level.String()] = count
	}
	return json.Marshal(result)
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
	for level := SeverityLevelSuccess; level <= SeverityLevelError; level++ {
		hgi.SeverityLevelCount[level] = 0
	}
	for _, issue := range issues {
		hgi.AddIssue(issue)
	}
	return hgi
}

func (hgi *HealthcheckGroupedIssues) AddIssue(issue *Door43HealthcheckIssue) {
	hgi.Issues[issue.IssueCode] = append(hgi.Issues[issue.IssueCode], issue)
	if issue.SeverityLevel > hgi.OverallSeverityLevel {
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
	IssueCodeNoMetadata:             "No metadata found for the repository",
	IssueCodeMetadataInvalid:        "Invalid metadata; file does not match schema",
	IssueCodeRelation:               "Relation is not the language of the resource",
	IssueCodePublisher:              "Publisher is still 'unfoldingWord'",
	IssueCodeTitle:                  "Title still contains 'unfoldingWord'",
	IssueCodeIdentifier:             "Identifier is not valid for the resource's subject",
	IssueCodeLanguage:               "Language is still English",
	IssueCodeIngredientTitle:        "Project title is still in English",
	IssueCodeIngredientMissing:      "Project file is missing",
	IssueCodeIngredientEmpty:        "Projct file is empty",
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
	IssueCodeTitle:                  "Resouce title in manifest.yaml still contains 'unfoldingWord'.",
	IssueCodeIdentifier:             "Identifier in manifest.yaml should not be **`%s`** for the subject **`%s`**.",
	IssueCodeLanguage:               "Language in manifest.yaml is still English **`en`**.",
	IssueCodeIngredientTitle:        "The title in for the project '**`%s`**' is still in English: **`%s`**",
	IssueCodeIngredientMissing:      "The file for project **`%s`** is does not exist in the repo: **`%s`**",
	IssueCodeIngredientEmpty:        "The file for project **`%s`** is empty: **`%s`**",
	IssueCodeReleaseNeeded:          "An error-free release needs to be published for the resource.",
	IssueCodeRelationMissing:        "Relations missing in manifest.yaml file: **`%s`**",
	IssueCodeOrigLangVersionMissing: "The relation **`%s`** is missing a version in the manifest.yaml file",
	IssueCodeOBSStoryMissing:        "The following stories are missing: **`%s`**",
	IssueCodeOBSStoryTitleMissing:   "The following stories are missing titles: **`%s`**",
	IssueCodeOBSWrongFrameCount:     "The following stories have the wrong frame count: **`%s`**",
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
	IssueCodeRelationMissing:        "Edit the manifRelations missing in <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and add the following relations: **`%s`**",
	IssueCodeOrigLangVersionMissing: "Edit the <a href=\"%s/src/branch/%s/manifest.yaml\" target=\"_blank\">manifest.yaml</a> file and add version used for the relation **`%s`**.",
	IssueCodeOBSStoryMissing:        "Add and translate the following stories: **`%s`**.",
	IssueCodeOBSStoryTitleMissing:   "Add titles to the following stories: **`%s`**. Translate the titles from the English version.",
	IssueCodeOBSWrongFrameCount:     "Add the missing frames to the following stories: **`%s`**. Check the English version for the missing frames.",
	IssueCodeOBSBibleRefenceMissing: "The following stories are missing Bible references: **`%s`**. Find the needed Bible references at the end of the English stories.",
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

func removeStringFromSlice(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// GetHealthcheck on Door43Metadata
func (dm *Door43Metadata) GetHealthcheck(ctx context.Context) *HealthcheckGroupedIssues {
	if dm.MetadataType != "rc" {
		return nil
	}
	dm.LoadRepo(ctx)
	issues := []*Door43HealthcheckIssue{}

	// Check if metadata is valid
	if dm.ValidationError != nil {
		item := &Door43HealthcheckIssue{
			IssueCode:     IssueCodeMetadataInvalid,
			SeverityLevel: SeverityLevelError,
			PositiveTitle: IssueCodeMetadataInvalid.IssuePositiveString(),
			NegativeTitle: IssueCodeMetadataInvalid.IssueNegativeString(),
			Details:       IssueCodeMetadataInvalid.IssueDetailsFormatString(),
			Suggestion:    fmt.Sprintf(IssueCodeMetadataInvalid.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dcs.ConvertValidationErrorToString(dm.ValidationError)),
		}
		issues = append(issues, item)
	}

	if dm.Repo.Owner.LowerName != "unfoldingword" && (dm.Publisher == "" || strings.HasPrefix(strings.TrimSpace(dm.Publisher), "unfoldingWord")) {
		item := &Door43HealthcheckIssue{
			IssueCode:     IssueCodePublisher,
			SeverityLevel: SeverityLevelError,
			PositiveTitle: IssueCodePublisher.IssuePositiveString(),
			NegativeTitle: IssueCodePublisher.IssueNegativeString(),
			Details:       IssueCodePublisher.IssueDetailsFormatString(),
			Suggestion:    fmt.Sprintf(IssueCodePublisher.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Repo.OwnerName),
		}
		issues = append(issues, item)
	}

	if dm.Repo.Owner.LowerName != "unfoldingword" && (dm.Title == "" || strings.HasPrefix(strings.TrimSpace(dm.Title), "unfoldingWord")) {
		item := &Door43HealthcheckIssue{
			IssueCode:     IssueCodeTitle,
			SeverityLevel: SeverityLevelError,
			PositiveTitle: IssueCodeTitle.IssuePositiveString(),
			NegativeTitle: IssueCodeTitle.IssueNegativeString(),
			Details:       IssueCodeTitle.IssueDetailsFormatString(),
			Suggestion:    fmt.Sprintf(IssueCodeTitle.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Title, strings.Join(strings.Fields(dm.Title)[1:], " ")),
		}
		issues = append(issues, item)
	}

	if dm.Abbreviation == "" || dm.Subject != "Bible" && dm.Subject != "Aligned Bible" {
		if identifiers, ok := dcs.SubjectToResourceMap[dm.Subject]; !ok || !slices.Contains(identifiers, dm.Abbreviation) {
			item := &Door43HealthcheckIssue{
				IssueCode:     IssueCodeIdentifier,
				SeverityLevel: SeverityLevelError,
				PositiveTitle: IssueCodeIdentifier.IssuePositiveString(),
				NegativeTitle: IssueCodeIdentifier.IssueNegativeString(),
				Details:       fmt.Sprintf(IssueCodeIdentifier.IssueDetailsFormatString(), dm.Abbreviation, dm.Subject),
				Suggestion:    fmt.Sprintf(IssueCodeIdentifier.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, dm.Abbreviation, dm.Subject, strings.Join(identifiers, ", ")),
			}
			issues = append(issues, item)
		}
	}

	if dm.Language == "en" && !strings.HasPrefix(dm.Repo.Name, "en_") {
		item := &Door43HealthcheckIssue{
			IssueCode:     IssueCodeLanguage,
			SeverityLevel: SeverityLevelWarning,
			PositiveTitle: IssueCodeLanguage.IssuePositiveString(),
			NegativeTitle: IssueCodeLanguage.IssueNegativeString(),
			Details:       IssueCodeLanguage.IssueDetailsFormatString(),
			Suggestion:    fmt.Sprintf(IssueCodeLanguage.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref),
		}
		issues = append(issues, item)
	}

	// Check for relations in other languages
	for _, relation := range dm.Relations {
		if relation.Language != dm.Language && relation.Language != "hbo" && relation.Language != "el-x-koine" && dm.Repo.Owner.LowerName != "unfoldingword" {
			item := &Door43HealthcheckIssue{
				IssueCode:     IssueCodeRelation,
				SeverityLevel: SeverityLevelError,
				PositiveTitle: IssueCodeRelation.IssuePositiveString(),
				NegativeTitle: IssueCodeRelation.IssueNegativeString(),
				Details:       fmt.Sprintf(IssueCodeRelation.IssueDetailsFormatString(), relation.FullRelation, dm.Language),
				Suggestion:    fmt.Sprintf(IssueCodeRelation.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, relation.FullRelation, dm.Language, relation.Identifier),
			}
			issues = append(issues, item)
		}
	}

	if dm.Ingredients != nil {
		doneIngredientTitle := false
		for _, ingredient := range dm.Ingredients {
			// Acts, Numbers and Deuteronomy are only in English and not other languages, so using those
			if !doneIngredientTitle && dm.Repo.Owner.LowerName != "unfoldingword" && (ingredient.Title == "" || (dm.Language != "en" && (ingredient.Title == "Numbers" || ingredient.Title == "Deuteronomy" || ingredient.Title == "Acts"))) {
				doneIngredientTitle = true
				item := &Door43HealthcheckIssue{
					IssueCode:     IssueCodeIngredientTitle,
					SeverityLevel: SeverityLevelError,
					PositiveTitle: IssueCodeIngredientTitle.IssuePositiveString(),
					NegativeTitle: IssueCodeIngredientTitle.IssueNegativeString(),
					Details:       fmt.Sprintf(IssueCodeIngredientTitle.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Title),
					Suggestion:    fmt.Sprintf(IssueCodeIngredientTitle.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Title),
				}
				issues = append(issues, item)
			}

			if !ingredient.Exists {
				item := &Door43HealthcheckIssue{
					IssueCode:     IssueCodeIngredientMissing,
					SeverityLevel: SeverityLevelError,
					PositiveTitle: IssueCodeIngredientMissing.IssuePositiveString(),
					NegativeTitle: IssueCodeIngredientMissing.IssueNegativeString(),
					Details:       fmt.Sprintf(IssueCodeIngredientMissing.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Path),
					Suggestion:    fmt.Sprintf(IssueCodeIngredientMissing.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Identifier, ingredient.Path),
				}
				issues = append(issues, item)
			}

			if ingredient.Exists && ingredient.Size == 0 && !ingredient.IsDir {
				item := &Door43HealthcheckIssue{
					IssueCode:     IssueCodeIngredientEmpty,
					SeverityLevel: SeverityLevelError,
					PositiveTitle: IssueCodeIngredientEmpty.IssuePositiveString(),
					NegativeTitle: IssueCodeIngredientEmpty.IssueNegativeString(),
					Details:       fmt.Sprintf(IssueCodeIngredientEmpty.IssueDetailsFormatString(), ingredient.Identifier, ingredient.Path),
					Suggestion:    fmt.Sprintf(IssueCodeIngredientEmpty.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, ingredient.Identifier, ingredient.Path),
				}
				issues = append(issues, item)
			}
		}
	}

	if dm.Subject == "TSV Translation Notes" {
		missingRelationIdentifiers := []string{"tw", "ta", "glt", "gst"}
		for _, relation := range dm.Relations {
			id := relation.Identifier
			if id == "ult" {
				id = "glt"
			}
			if id == "ust" {
				id = "gst"
			}
			if slices.Contains(missingRelationIdentifiers, id) {
				missingRelationIdentifiers = removeStringFromSlice(missingRelationIdentifiers, id)
				continue
			}
		}
		if len(missingRelationIdentifiers) > 0 {
			item := &Door43HealthcheckIssue{
				IssueCode:     IssueCodeRelationMissing,
				SeverityLevel: SeverityLevelWarning,
				PositiveTitle: IssueCodeRelationMissing.IssuePositiveString(),
				NegativeTitle: IssueCodeRelationMissing.IssueNegativeString(),
				Details:       fmt.Sprintf(IssueCodeRelationMissing.IssueDetailsFormatString(), strings.Join(missingRelationIdentifiers, ", ")),
				Suggestion:    fmt.Sprintf(IssueCodeRelationMissing.IssueSuggestionFormatString(), dm.Repo.Link(), dm.Ref, strings.Join(missingRelationIdentifiers, ", ")),
			}
			issues = append(issues, item)
		}
	}

	if dm.Relations != nil {
		for _, relation := range dm.Relations {
			// Check if relation catalog exists
			ref := relation.Version
			isVersion := false
			if ref == "" {
				ref = dm.Repo.DefaultBranch
			} else {
				isVersion = true
			}
			catalogURL := fmt.Sprintf("%sapi/v1/catalog/entry/%s/%s_%s/%s",
				setting.AppURL, dm.Repo.Owner.Name, relation.Language, relation.Identifier, ref)
			resp, err := http.Get(catalogURL)
			if err != nil {
				log.Error("Error fetching catalog for relation %s: %v", relation.FullRelation, err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 && isVersion {
				// Try again with default branch if a version was specified and not found
				ref = "v" + ref
				catalogURL = fmt.Sprintf("%sapi/v1/catalog/entry/%s/%s_%s/%s",
					setting.AppURL, dm.Repo.Owner.Name, relation.Language, relation.Identifier, ref)
				resp, err = http.Get(catalogURL)
				if err != nil {
					log.Error("Error fetching catalog for relation %s: %v", relation.FullRelation, err)
					continue
				}
				defer resp.Body.Close()
			}

			if resp.StatusCode != 200 {
				item := &Door43HealthcheckIssue{
					IssueCode:     IssueCodeRelationMissing,
					SeverityLevel: SeverityLevelError,
					PositiveTitle: IssueCodeRelationMissing.IssuePositiveString(),
					NegativeTitle: IssueCodeRelationMissing.IssueNegativeString(),
					Details:       fmt.Sprintf("Relation %s does not exist in the DCS catalog", relation.FullRelation),
					Suggestion:    fmt.Sprintf("Verify that the relation %s exists and is properly cataloged", relation.FullRelation),
				}
				issues = append(issues, item)
				continue
			}

			// Parse catalog response to check ingredients
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Error("Error reading catalog response for relation %s: %v", relation.FullRelation, err)
				continue
			}

			var catalogData struct {
				Subject     string `json:"subject"`
				Ingredients []struct {
					Identifier string `json:"identifier"`
				} `json:"ingredients"`
			}

			if err := json.Unmarshal(body, &catalogData); err != nil {
				log.Error("Error parsing catalog JSON for relation %s: %v", relation.FullRelation, err)
				continue
			}

			// Check if each ingredient exists in the relation's catalog
			if dm.Ingredients != nil && (strings.HasPrefix(dm.Subject, "TSV ") || strings.Contains(dm.Subject, "Bible")) &&
				!strings.Contains(dm.Subject, "OBS") && (strings.HasPrefix(catalogData.Subject, "TSV ") || strings.Contains(catalogData.Subject, "Bible")) &&
				!strings.Contains(catalogData.Subject, "OBS") {
				for _, ingredient := range dm.Ingredients {
					if ingredient.Identifier == "frt" || ingredient.Identifier == "bak" {
						continue
					}
					found := false
					for _, catalogIngredient := range catalogData.Ingredients {
						if ingredient.Identifier == catalogIngredient.Identifier {
							found = true
							break
						}
					}

					if !found {
						item := &Door43HealthcheckIssue{
							IssueCode:     IssueCodeRelationMissing,
							SeverityLevel: SeverityLevelWarning,
							PositiveTitle: IssueCodeRelationMissing.IssuePositiveString(),
							NegativeTitle: IssueCodeRelationMissing.IssueNegativeString(),
							Details:       fmt.Sprintf("Relation %s does not contain book %s", relation.FullRelation, ingredient.Identifier),
							Suggestion:    fmt.Sprintf("Ensure that the relation %s includes the book %s", relation.FullRelation, ingredient.Identifier),
						}
						issues = append(issues, item)
					}
				}
			}
		}
	}

	healthcheckGroupedIssues := NewHealthcheckGroupedIssues(issues)

	dm.LoadRepo(ctx)
	dm.Repo.LoadLatestDMs(ctx)
	if dm.Ref == dm.Repo.DefaultBranch {
		if healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelError] > 0 {
			item := &Door43HealthcheckIssue{
				IssueCode:     IssueCodeReleaseNeeded,
				SeverityLevel: SeverityLevelError,
				PositiveTitle: IssueCodeReleaseNeeded.IssuePositiveString(),
				NegativeTitle: IssueCodeReleaseNeeded.IssueNegativeString(),
				Details:       IssueCodeReleaseNeeded.IssueDetailsFormatString(),
				Suggestion:    "Fix all the errors above. Then make a release with <a href=\"https://gateway-admin.netlify.app/\" target=\"_blank\">gatewayAdmin</a>.",
			}
			healthcheckGroupedIssues.AddIssue(item)
		} else if dm.Repo.LatestProdDM == nil {
			item := &Door43HealthcheckIssue{
				IssueCode:     IssueCodeReleaseNeeded,
				SeverityLevel: SeverityLevelError,
				PositiveTitle: IssueCodeReleaseNeeded.IssuePositiveString(),
				NegativeTitle: IssueCodeReleaseNeeded.IssueNegativeString(),
				Details:       IssueCodeReleaseNeeded.IssueDetailsFormatString(),
				Suggestion:    "No release exists for this resource. Make a release with <a href=\"https://gateway-admin.netlify.app/\" target=\"_blank\">gatewayAdmin</a>.",
			}
			healthcheckGroupedIssues.AddIssue(item)
		} else {
			var releaseHealthcheckGroupedIssues *HealthcheckGroupedIssues
			if dm.Repo.LatestProdDM != nil {
				releaseHealthcheckGroupedIssues = dm.Repo.LatestProdDM.GetHealthcheck(ctx)
			}

			if healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelError] == 0 && releaseHealthcheckGroupedIssues != nil && releaseHealthcheckGroupedIssues.SeverityLevelCount[SeverityLevelError] > 0 {
				item := &Door43HealthcheckIssue{
					IssueCode:     IssueCodeReleaseNeeded,
					SeverityLevel: SeverityLevelError,
					PositiveTitle: IssueCodeReleaseNeeded.IssuePositiveString(),
					NegativeTitle: IssueCodeReleaseNeeded.IssueNegativeString(),
					Details:       IssueCodeReleaseNeeded.IssueDetailsFormatString(),
					Suggestion:    fmt.Sprintf(IssueCodeReleaseNeeded.IssueSuggestionFormatString(), "all", dm.Ref, SeverityLevelError.String()),
				}
				healthcheckGroupedIssues.AddIssue(item)
			} else if releaseHealthcheckGroupedIssues != nil && healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelWarning] < releaseHealthcheckGroupedIssues.SeverityLevelCount[SeverityLevelWarning] {
				item := &Door43HealthcheckIssue{
					IssueCode:     IssueCodeReleaseNeeded,
					SeverityLevel: SeverityLevelInfo,
					PositiveTitle: IssueCodeReleaseNeeded.IssuePositiveString(),
					NegativeTitle: IssueCodeReleaseNeeded.IssueNegativeString(),
					Details:       IssueCodeReleaseNeeded.IssueDetailsFormatString(),
					Suggestion:    fmt.Sprintf(IssueCodeReleaseNeeded.IssueSuggestionFormatString(), "some or all", dm.Ref, SeverityLevelWarning.String()),
				}
				healthcheckGroupedIssues.AddIssue(item)
			}
		}
	}

	if dm.HealthcheckSeverity != healthcheckGroupedIssues.OverallSeverityLevel ||
		dm.HealthcheckCounts == nil ||
		dm.HealthcheckCounts[SeverityLevelError] != healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelError] ||
		dm.HealthcheckCounts[SeverityLevelWarning] != healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelWarning] ||
		dm.HealthcheckCounts[SeverityLevelInfo] != healthcheckGroupedIssues.SeverityLevelCount[SeverityLevelInfo] {
		dm.HealthcheckSeverity = healthcheckGroupedIssues.OverallSeverityLevel
		dm.HealthcheckCounts = healthcheckGroupedIssues.SeverityLevelCount
		if _, err := db.GetEngine(ctx).ID(dm.ID).Cols("healthcheck_severity", "healthcheck_counts").Update(dm); err != nil {
			log.Error("Error updating healthcheck severity and counts: %v", err)
			return nil
		}
	}

	return healthcheckGroupedIssues
}
