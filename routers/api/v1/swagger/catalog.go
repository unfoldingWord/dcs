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

// CatalogValidation
// swagger:response CatalogValidation
type swaggerResponseCatalogValidation struct {
	// in:body
	Body map[string]interface{} `json:"body"`
}

// Language
// swagger:response Language
type swaggerResponseLanguage struct {
	// in:body
	Body map[string]interface{} `json:"body"`
}
