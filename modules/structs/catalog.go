// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	Repo                   *Repository   `json:"repo"`
	Release                *Release      `json:"release"`
	TarballURL             string        `json:"tarbar_url"`
	ZipballURL             string        `json:"zipball_url"`
	GitTreesURL            string        `json:"git_trees_url"`
	ContentsURL            string        `json:"contents_url"`
	Language               string        `json:"language"`
	LanguageTitle          string        `json:"language_title"`
	LanguageDir            string        `json:"language_direction"`
	LanguageIsGL           bool          `json:"language_is_gl"`
	Subject                string        `json:"subject"`
	Title                  string        `json:"title"`
	Ref                    string        `json:"branch_or_tag_name"`
	RefType                string        `json:"ref_type"`
	CommitID               string        `json:"commit_id"`
	Stage                  string        `json:"stage"`
	MetadataURL            string        `json:"metadata_url"`
	MetadataJSONURL        string        `json:"metadata_json_url"`
	MetadataAPIContentsURL string        `json:"metadata_api_contents_url"`
	MetadataVersion        string        `json:"metadata_version"`
	MetadataType           string        `json:"metadata_type"`
	ContentFormat          string        `json:"content_format"`
	Released               time.Time     `json:"released"`
	Ingredients            []*Ingredient `json:"ingredients,omitempty"`
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
}

// CatalogSearchResults results of a successful catalog search
type CatalogSearchResults struct {
	OK          bool            `json:"ok"`
	Data        []*CatalogEntry `json:"data"`
	LastUpdated time.Time       `json:"last_updated"`
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
	Released    time.Time `json:"released"`
	ZipballURL  string    `json:"zipball_url"`
	TarballURL  string    `json:"tarball_url"`
	GitTreesURL string    `json:"git_trees_url"`
	ContentsURL string    `json:"contents_url"`
}
