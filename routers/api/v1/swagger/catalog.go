// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// CatalogSearchResults
// swagger:response CatalogSearchResults
type swaggerResponseCatalogSearchResults struct {
	// in:body
	Body api.CatalogSearchResults `json:"body"`
}

// CatalogEntry
// swagger:response CatalogEntry
type swaggerResponseCatalogEntry struct {
	// in:body
	Body api.CatalogEntry `json:"body"`
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
