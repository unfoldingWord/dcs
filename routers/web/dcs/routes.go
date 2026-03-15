// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/repo"
	"code.gitea.io/gitea/services/context"
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

// RegisterDCSRepoWebRoutes registers DCS repo-scoped web routes (healthcheck, metadata)
func RegisterDCSRepoWebRoutes(m *web.Router) {
	m.Group("/healthcheck", func() {
		m.Get("", repo.GetRepoHealthcheck)
		m.Get("/update", repo.UpdateDoor43Metadata)
		m.Post("/update", repo.UpdateDoor43Metadata)
	})
	m.Group("/metadata", func() {
		m.Get("", repo.GetAllRepoDoor43Metadata)
		m.Get("/update", repo.UpdateDoor43Metadata)
		m.Post("/update", repo.UpdateDoor43Metadata)
	})
}
