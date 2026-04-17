// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/admin"
	"code.gitea.io/gitea/routers/api/v1/catalog"
	"code.gitea.io/gitea/routers/api/v1/repo"
	"code.gitea.io/gitea/services/context"
)

// RegisterDCSAPIRoutes registers top-level DCS API routes (languages, catalog)
func RegisterDCSAPIRoutes(m *web.Router,
	repoAssignment func() func(ctx *context.APIContext),
	reqRepoReader func(unitType unit.Type) func(ctx *context.APIContext),
) {
	m.Group("/languages", func() {
		m.Get("/langnames.json", ServeLangnamesJSON)
		m.Get("/langnames_keyed.json", ServeLangnamesJSONKeyed)
	})
	m.Group("/catalog", func() {
		m.Get("", catalog.Search)
		m.Group("/list", func() {
			m.Get("/subjects", catalog.ListCatalogSubjects)
			m.Get("/owners", catalog.ListCatalogOwners)
			m.Get("/languages", catalog.ListCatalogLanguages)
			m.Get("/metadata-types", catalog.ListCatalogMetadataTypes)
		})
		m.Group("/search", func() {
			m.Get("", catalog.Search)
			// The below are deprecated
			m.Group("/{username}", func() {
				m.Get("", catalog.SearchOwner)
				m.Group("/{reponame}", func() {
					m.Get("", catalog.SearchRepo)
				}, repoAssignment())
			})
		})
		m.Group("", func() {
			m.Group("/entry/{username}/{reponame}", func() {
				m.Get("/{ref}/metadata", catalog.GetCatalogMetadataOLD) // DEPRECATED
				m.Get("/*", catalog.GetCatalogEntry)
			})
			m.Get("/metadata/{username}/{reponame}/*", catalog.GetCatalogMetadata)
			m.Get("/validation/{username}/{reponame}/*", catalog.GetCatalogValidation)
			m.Get("/bp/{username}/{reponame}", reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(), catalog.GetCatalogBookPackage)
			m.Get("/bp/{username}/{reponame}/*", reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(), catalog.GetCatalogBookPackage)
		}, repoAssignment())
	})
}

// RegisterDCSRepoAPIRoutes registers DCS repo-scoped API routes (healthcheck)
func RegisterDCSRepoAPIRoutes(m *web.Router) {
	m.Get("/healthcheck", repo.GetHealthcheck)
}

// RegisterDCSAdminAPIRoutes registers DCS admin API routes (spam users)
func RegisterDCSAdminAPIRoutes(m *web.Router) {
	m.Group("/spam", func() {
		m.Get("", admin.ListSpamUsers)
		m.Delete("", admin.DeleteSpamUsers)
	})
}
