// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// CatalogEntry represents a repository's metadata of a tag or default branch as an entry of the catalog
type CatalogEntry struct {
	ID                     int64         `json:"id"`
	Self                   string        `json:"url"`
	Name                   string        `json:"name"`
	Owner                  string        `json:"owner"`
	FullName               string        `json:"full_name"`
	Repo                   *Repository   `json:"repo,omitempty"`
	Release                *Release      `json:"release,omitempty"`
	TarballURL             string        `json:"tarbar_url"`
	ZipballURL             string        `json:"zipball_url"`
	GitTreesURL            string        `json:"git_trees_url"`
	ContentsURL            string        `json:"contents_url"`
	Language               string        `json:"language"`
	LanguageTitle          string        `json:"language_title"`
	LanguageDir            string        `json:"language_direction"`
	LanguageIsGL           bool          `json:"language_is_gl"`
	Subject                string        `json:"subject"`
	FlavorType             string        `json:"flavor_type"`
	Flavor                 string        `json:"flavor"`
	Abbreviation           string        `json:"abbreviation"`
	Title                  string        `json:"title"`
	Ref                    string        `json:"branch_or_tag_name"`
	RefType                string        `json:"ref_type"`
	CommitSHA              string        `json:"commit_sha"`
	Stage                  string        `json:"stage"`
	MetadataURL            string        `json:"metadata_url"`
	MetadataJSONURL        string        `json:"metadata_json_url"`
	MetadataAPIContentsURL string        `json:"metadata_api_contents_url"`
	MetadataType           string        `json:"metadata_type"`
	MetadataVersion        string        `json:"metadata_version"`
	ContentFormat          string        `json:"content_format"`
	Released               time.Time     `json:"released"`
	Ingredients            []*Ingredient `json:"ingredients,omitempty"`
	Books                  []string      `json:"books,omitempty"`
	Relations              []*Relation   `json:"relations"`
	// AttachmentTypes the kinds of content available in the entry's release attachments; null if the entry is not a release
	AttachmentTypes     *CatalogAttachmentTypes `json:"attachment_types"`
	IsValid             bool                    `json:"is_valid"`
	ValidationErrorsURL string                  `json:"validation_errors_url"`
	// HealthcheckSeverity the overall severity of this entry's health check: success, info, warning or error; empty if never checked
	HealthcheckSeverity string `json:"healthcheck_severity"`
	// IsHealthy true when this entry passed its health check, ignoring warnings
	IsHealthy bool `json:"is_healthy"`
	// IsHealthyWithoutWarnings true when this entry passed its health check with no warnings
	IsHealthyWithoutWarnings bool `json:"is_healthy_without_warnings"`
	// HealthcheckURL the API URL for this entry's full health check results
	HealthcheckURL string `json:"healthcheck_url"`
}

// CatalogAttachmentTypes the kinds of content found in the release attachments of a catalog entry
type CatalogAttachmentTypes struct {
	PDF    bool `json:"pdf"`
	Audio  bool `json:"audio"`
	Video  bool `json:"video"`
	Stream bool `json:"stream"`
	Other  bool `json:"other"`
}

// Ingredient is a single project of a resource
type Ingredient struct {
	Categories     []string `json:"categories"`
	Identifier     string   `json:"identifier"`
	Path           string   `json:"path"`
	Sort           int      `json:"sort"`
	Title          string   `json:"title"`
	Versification  string   `json:"versification"`
	AlignmentCount *int     `json:"alignment_count,omitempty"`
	Exists         bool     `json:"exists"`
	IsDir          bool     `json:"is_dir"`
	Size           int64    `json:"size"`
}

// Relation is a single relation of a resource
type Relation struct {
	FullRelation string `json:"full_relation"`
	Language     string `json:"lang"`
	Identifier   string `json:"identifier"`
	Version      string `json:"version"`
}

// CatalogSearchResults results of a successful catalog search
type CatalogSearchResults struct {
	OK          bool            `json:"ok"`
	Data        []*CatalogEntry `json:"data"`
	LastUpdated time.Time       `json:"last_updated"`
}

// CatalogStats aggregate counts of the catalog entries matching a catalog search.
// Entry-based counts are repo counts over the latest entry per repo and stage; the
// has_* media counts are repo counts considering every release of the stage(s).
type CatalogStats struct {
	EntryCount      int64 `json:"entry_count"`
	LangCount       int64 `json:"lang_count"`
	LangLtrCount    int64 `json:"lang_ltr_count"`
	LangRtlCount    int64 `json:"lang_rtl_count"`
	SubjectCount    int64 `json:"subject_count"`
	FlavorTypeCount int64 `json:"flavor_type_count"`
	FlavorCount     int64 `json:"flavor_count"`
	OwnerCount      int64 `json:"owner_count"`
	RepoCount       int64 `json:"repo_count"`
	TsCount         int64 `json:"ts_count"`
	TcCount         int64 `json:"tc_count"`
	RcCount         int64 `json:"rc_count"`
	SbCount         int64 `json:"sb_count"`
	HasPDF          int64 `json:"has_pdf"`
	HasAudio        int64 `json:"has_audio"`
	HasVideo        int64 `json:"has_video"`
	HasStream       int64 `json:"has_stream"`
	HasOther        int64 `json:"has_other"`
	HasAttachment   int64 `json:"has_attachment"`
}

// CatalogStatsExt CatalogStats plus the healthcheck counts and the entry counts per
// subject, flavor type, flavor, owner, language and metadata type
type CatalogStatsExt struct {
	CatalogStats
	HealthcheckSuccessCount int64 `json:"healthcheck_success_count"`
	HealthcheckInfoCount    int64 `json:"healthcheck_info_count"`
	HealthcheckWarningCount int64 `json:"healthcheck_warning_count"`
	HealthcheckErrorCount   int64 `json:"healthcheck_error_count"`
	NoHealthcheckCount      int64 `json:"no_healthcheck_count"`
	// IsHealthyCount repos whose matching entry passed its health check, ignoring warnings (severity success, info or warning)
	IsHealthyCount int64 `json:"is_healthy_count"`
	// IsHealthyWithoutWarningsCount repos whose matching entry passed its health check with no warnings (severity success or info)
	IsHealthyWithoutWarningsCount int64            `json:"is_healthy_without_warnings_count"`
	Subjects                      map[string]int64 `json:"subjects"`
	FlavorTypes                   map[string]int64 `json:"flavor_types"`
	Flavors                       map[string]int64 `json:"flavors"`
	Owners                        map[string]int64 `json:"owners"`
	Languages                     map[string]int64 `json:"languages"`
	MetadataTypes                 map[string]int64 `json:"metadata_types"`
}

// CatalogVersionEndpoints Info on the versions of the catalog
type CatalogVersionEndpoints struct {
	Latest   string            `json:"latest"`
	Versions map[string]string `json:"versions"`
}

// CatalogStages a repo's catalog stages
type CatalogStages struct {
	Production    *CatalogStage `json:"prod"`
	PreProduction *CatalogStage `json:"preprod"`
	Latest        *CatalogStage `json:"latest"`
}

// CatalogStage a repo's catalog stage metadata
type CatalogStage struct {
	Ref         string    `json:"branch_or_tag_name"`
	ReleaseURL  *string   `json:"release_url"`
	CommitSHA   string    `json:"commit_sha"`
	Released    time.Time `json:"released"`
	ZipballURL  string    `json:"zipball_url"`
	TarballURL  string    `json:"tarball_url"`
	GitTreesURL string    `json:"git_trees_url"`
	ContentsURL string    `json:"contents_url"`
}
