// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import "time"

// CatalogV3 represents the root of the v3 Catalog
type CatalogV3 struct {
	Catalogs    []map[string]string  `json:"catalogs"`
	Languages   []*CatalogV3Language `json:"languages"`
	LastUpdated time.Time            `json:"last_updated"`
}

// CatalogV3Language represents a language in the catalog v3 languages array
type CatalogV3Language struct {
	Identifier  string               `json:"identifier"`
	Title       string               `json:"title"`
	Direction   string               `json:"direction"`
	Resources   []*CatalogV3Resource `json:"resources"`
	LastUpdated time.Time            `json:"last_updated"`
}

// CatalogV3Pivoted represents the root of the v3 Pivoted Catalog
type CatalogV3Pivoted struct {
	Catalogs    []map[string]string `json:"catalogs"`
	Subjects    []*CatalogV3Subject `json:"subjects"`
	LastUpdated time.Time           `json:"last_updated"`
}

// CatalogV3Subjects represents a subject & language combination in the catalog v3 languages array
type CatalogV3Subject struct {
	Subject     string               `json:"subject"`
	Identifier  string               `json:"identifier"`
	Language    string               `json:"language"`
	Resources   []*CatalogV3Resource `json:"resources"`
	Direction   string               `json:"direction"`
	Title       string               `json:"title"`
	LastUpdated time.Time            `json:"last_updated"`
}

// CatalogV3Resource represents a resource in the catalog v3 resources array
type CatalogV3Resource struct {
	Identifier  string                   `json:"identifier"`
	Title       string                   `json:"title"`
	Subject     string                   `json:"subject"`
	Version     string                   `json:"version"`
	Checking    map[string]interface{}   `json:"checking"`
	Comment     string                   `json:"comment"`
	Contributor []interface{}            `json:"contributor"`
	Creator     string                   `json:"creator"`
	Description string                   `json:"description"`
	Formats     []map[string]interface{} `json:"formats"`
	Issued      time.Time                `json:"issued"`
	Modified    time.Time                `json:"modified"`
	Projects    []interface{}            `json:"projects"`
	Publisher   string                   `json:"publisher"`
	Relation    []interface{}            `json:"relation"`
	Rights      string                   `json:"rights"`
	Source      []interface{}            `json:"source"`
	Owner       string                   `json:"owner"`
	RepoName    string                   `json:"repo"`
}

// CatalogV4 represents a repository's metadata of a tag or default branch
type CatalogV4 struct {
	ID                     int64         `json:"id"`
	Self                   string        `json:"url"`
	Repo                   string        `json:"repo"`
	Owner                  string        `json:"owner"`
	RepoURL                string        `json:"repo_url"`
	ReleaseURL             string        `json:"release_url"`
	TarballURL             string        `json:"tarbar_url"`
	ZipballURL             string        `json:"zipball_url"`
	Language               string        `json:"lang_code"`
	Subject                string        `json:"subject"`
	Title                  string        `json:"title"`
	BranchOrTag            string        `json:"branch_or_tag_name"`
	Stage                  string        `json:"stage"`
	MetadataURL            string        `json:"metadata_url"`
	MetadataJSONURL        string        `json:"metadata_json_url"`
	MetadataAPIContentsURL string        `json:"metadata_api_contents_url"`
	MetadataVersion        string        `json:"metadata_version"`
	Released               time.Time     `json:"released"`
	Books                  []string      `json:"books"`
	Ingredients            []interface{} `json:"ingredients,omitempty"`
}

// CatalogV5 represents a repository's metadata of a tag or default branch for V5
type CatalogV5 struct {
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
	Subject                string        `json:"subject"`
	Title                  string        `json:"title"`
	BranchOrTag            string        `json:"branch_or_tag_name"`
	Stage                  string        `json:"stage"`
	MetadataURL            string        `json:"metadata_url"`
	MetadataJSONURL        string        `json:"metadata_json_url"`
	MetadataAPIContentsURL string        `json:"metadata_api_contents_url"`
	MetadataVersion        string        `json:"metadata_version"`
	Released               time.Time     `json:"released"`
	Books                  []string      `json:"books"`
	Ingredients            []interface{} `json:"ingredients,omitempty"`
}

// CatalogSearchResultsV3 results of a successful search for V3
type CatalogSearchResultsV3 struct {
	OK          bool         `json:"ok"`
	Data        []*CatalogV3 `json:"data"`
	LastUpdated time.Time    `json:"last_updated"`
}

// CatalogSearchResultsPivotedV3 results of a successful search for V3 pivoted
type CatalogSearchResultsPivotedV3 struct {
	OK          bool                `json:"ok"`
	Data        []*CatalogV3Pivoted `json:"data"`
	LastUpdated time.Time           `json:"last_updated"`
}

// CatalogSearchResultsV4 results of a successful search for V4
type CatalogSearchResultsV4 struct {
	OK          bool         `json:"ok"`
	Data        []*CatalogV4 `json:"data"`
	LastUpdated time.Time    `json:"last_updated"`
}

// CatalogSearchResultsV5 results of a successful search for V5
type CatalogSearchResultsV5 struct {
	OK          bool         `json:"ok"`
	Data        []*CatalogV5 `json:"data"`
	LastUpdated time.Time    `json:"last_updated"`
}

// CatalogVersionEndpoints Info on the versions of the catalog
type CatalogVersionEndpoints struct {
	Latest   string            `json:"latest"`
	Versions map[string]string `json:"versions"`
}

// CatalogVersionEndpointsResponse response with the endpoints for all versions of the catalog
type CatalogVersionEndpointsResponse struct {
	OK   bool                       `json:"ok"`
	Data []*CatalogVersionEndpoints `json:"data"`
}

// CatalogStages a repo's catalog stages
type CatalogStages struct {
	Production    *CatalogStage `json:"prod"`
	PreProduction *CatalogStage `json:"preprod"`
	Draft         *CatalogStage `json:"draft"`
	Latest        *CatalogStage `json:"latest"`
}

// CatalogStage a repo's catalog stage metadata
type CatalogStage struct {
	Tag         string    `json:"branch_or_tag_name"`
	ReleaseURL  *string   `json:"release_url"`
	Released    time.Time `json:"released"`
	ZipballURL  string    `json:"zipball_url"`
	TarballURL  string    `json:"tarball_url"`
	GitTreesURL string    `json:"git_trees_url"`
	ContentsURL string    `json:"contents_url"`
}
