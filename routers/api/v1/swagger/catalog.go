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

// CatalogStats
// swagger:response CatalogStats
type swaggerResponseCatalogStats struct {
	// in:body
	Body api.CatalogStats `json:"body"`
}

// CatalogStatsExt
// swagger:response CatalogStatsExt
type swaggerResponseCatalogStatsExt struct {
	// in:body
	Body api.CatalogStatsExt `json:"body"`
}

// CatalogMetadata
// swagger:response CatalogMetadata
type swaggerResponseCatalogMetadata struct {
	// in:body
	Body map[string]any `json:"body"`
}

// CatalogValidation
// swagger:response CatalogValidation
type swaggerResponseCatalogValidation struct {
	// in:body
	Body map[string]any `json:"body"`
}

// Language
// swagger:response Language
type swaggerResponseLanguage struct {
	// in:body
	Body map[string]any `json:"body"`
}

// Door43Healthcheck
// swagger:response Door43Healthcheck
type swaggerResponseDoor43Healthcheck struct {
	// in:body
	Body map[string]any `json:"body"`
}
