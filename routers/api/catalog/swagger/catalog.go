// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// CatalogSearchResultsV3
// swagger:response CatalogSearchResultsV3
type swaggerResponseCatalogSearchResultsV3 struct {
	// in:body
	Body api.CatalogSearchResultsV3 `json:"body"`
}

// CatalogSearchResultsPivotedV3
// swagger:response CatalogSearchResultsPivotedV3
type swaggerResponseCatalogSearchResultsPivotedV3 struct {
	// in:body
	Body api.CatalogSearchResultsPivotedV3 `json:"body"`
}

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
	Body api.CatalogV4 `json:"body"`
}

// CatalogSearchResultsV5
// swagger:response CatalogSearchResultsV5
type swaggerResponseCatalogSearchResultsV5 struct {
	// in:body
	Body api.CatalogSearchResultsV5 `json:"body"`
}

// CatalogEntryV5
// swagger:response CatalogEntryV5
type swaggerResponseCatalogEntryV5 struct {
	// in:body
	Body api.CatalogV5 `json:"body"`
}

// CatalogMetadata
// swagger:response CatalogMetadata
type swaggerResponseCatalogMetadata struct {
	// in:body
	Body map[string]interface{} `json:"body"`
}

// CatalogVersionEndpointsResponse
// swagger:response CatalogVersionEndpointsResponse
type swaggerResponseCatalogVersionEndpointsResponse struct {
	// in:body
	Body api.CatalogVersionEndpointsResponse `json:"body"`
}
