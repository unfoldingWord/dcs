// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// CatalogSearchResultsV4
// swagger:response CatalogSearchResultsV4
type swaggerResponseCatalogSearchResultsV4 struct {
	// in:body
	Body api.CatalogSearchResultsV4 `json:"body"`
}

// CatalogEntryV4
// swagger:response CatalogEntryV4
type swaggerResponseCatalogEntryV4 struct {
	// in:body
	Body api.Door43MetadataV4 `json:"body"`
}

// CatalogMetadata
// swagger:response CatalogMetadata
type swaggerResponseCatalogMetadata struct {
	// in:body
	Body map[string]interface{} `json:"body"`
}

// CatalogVersionList
// swagger:response CatalogVersionList
type swaggerResponseCatalogVersionList struct {
	// in:body
	Body map[string]interface{} `json:"body"`
}
