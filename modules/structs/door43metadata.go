// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Door43Metadata represents a repository's metadata of a tag or default branch
type Door43Metadata struct {
	ID              int64         `json:"id"`
	Self            string        `json:"url"`
	RepoURL         string        `json:"repo_url"`
	ReleaseURL      string        `json:"release_url"`
	Language        string        `json:"lang_code"`
	Subject         string        `json:"subject"`
	Title           string        `json:"title"`
	Books           []string      `json:"books"`
	Tag             string        `json:"branch_or_tag_name"`
	Stage           string        `json:"stage"`
	Ingredients     []interface{} `json:"ingredients"`
	MetadataURL     string        `json:"metadata_url"`
	MetadataAPIURL  string        `json:"metadata_api_url"`
	MetadataVersion string        `json:"metadata_version"`
	Released        string        `json:"released"`
}

// CatalogSearchResults results of a successful search
type CatalogSearchResults struct {
	OK   bool              `json:"ok"`
	Data []*Door43Metadata `json:"data"`
}
