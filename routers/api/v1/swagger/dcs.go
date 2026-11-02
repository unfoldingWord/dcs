// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

// LanguLangnamesJSON
// swagger:response LangnamesJSON
type swaggerResponseLangnamesJSON struct {
	// in:body
	Body map[string]any `json:"body"`
}

// Upstream dropped these for "default: error" (go-gitea/gitea#40048); DCS endpoints still return them.

// APINotFound is a not found empty response
// swagger:response notFound
type swaggerResponseNotFound struct{}

// APIConflict is a conflict empty response
// swagger:response conflict
type swaggerResponseConflict struct{}

// APIForbiddenError is a forbidden error response
// swagger:response forbidden
type swaggerResponseForbidden struct {
	Message string `json:"message"`
	URL     string `json:"url"`
}

// APIValidationError is error format response related to input validation
// swagger:response validationError
type swaggerResponseValidationError struct {
	Message string `json:"message"`
	URL     string `json:"url"`
}
