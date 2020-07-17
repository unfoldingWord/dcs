// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// CatalogSearchResults
// swagger:response CatalogSearchResults
type swaggerResponseCatalogSearchResults struct {
	// in:body
	Body api.CatalogSearchResultsV4 `json:"body"`
}

// CatalogEntry
// swagger:response CatalogEntry
type swaggerResponseCatalogEntry struct {
	// in:body
	Body api.Door43MetadataV4 `json:"body"`
}

// CatalogMetadata
// swagger:response CatalogMetadata
type swaggerResponseCatalogMetadata struct {
	// in:body
	Body map[string]interface{} `json:"body"`
}
