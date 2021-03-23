// Copyright 2021 The unfoldingWord Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// CatalogMetadata metadata for a repo's catalog entry
type CatalogMetadata struct {
	Production    *CatalogStageMetadata `json:"prod"`
	PreProduction *CatalogStageMetadata `json:"preprod"`
	Draft         *CatalogStageMetadata `json:"draft"`
	Latest        *CatalogStageMetadata `json:"latest"`
}

// CatalogStageMetadata metadata for a stage of a repo's catalog entry
type CatalogStageMetadata struct {
	Tag        string  `json:"branch_or_tag_name"`
	ReleaseURL *string `json:"release_url"`
}
