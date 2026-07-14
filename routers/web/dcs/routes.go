// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"gitea.dev/modules/web"
	"gitea.dev/routers/web/repo"
	"gitea.dev/services/context"
)

// RegisterDCSWebRoutes registers top-level DCS web routes (about, tools, catalog, healthcheck dashboard)
func RegisterDCSWebRoutes(m *web.Router, optSignIn, reqSignIn func(ctx *context.Context)) {
	m.Get("/about", About)
	m.Get("/tools", Tools)
	m.Get("/hc-dash", reqSignIn, HealthcheckDashboard)
	m.Group("/catalog", func() {
		m.Get("", Catalog)
	}, optSignIn)
}

// RegisterDCSRepoWebRoutes registers DCS repo-scoped web routes (metadata, healthcheck)
func RegisterDCSRepoWebRoutes(m *web.Router) {
	m.Group("/metadata", func() {
		m.Get("", repo.GetRepoMetadata)
		m.Get("/all", repo.GetAllRepoDoor43Metadata)
		m.Get("/update", repo.UpdateDoor43Metadata)
		m.Post("/update", repo.UpdateDoor43Metadata)
	})
	m.Get("/healthcheck", repo.GetRepoHealthcheck)
}
