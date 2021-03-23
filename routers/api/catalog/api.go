// Copyright 2021 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package v4 Catalog API.
//
// This documentation describes the DCS Catalog API.
//
//     Schemes: http, https
//     BasePath: /api/catalog/v4
//     Version: 4
//     License: MIT http://opensource.org/licenses/MIT
//
//     Consumes:
//     - application/json
//     - text/plain
//
//     Produces:
//     - application/json
//     - text/html
//
//     Security:
//     - BasicAuth :
//     - Token :
//     - AccessToken :
//     - AuthorizationHeaderToken :
//     - SudoParam :
//     - SudoHeader :
//
//     SecurityDefinitions:
//     BasicAuth:
//          type: basic
//     Token:
//          type: apiKey
//          name: token
//          in: query
//     AccessToken:
//          type: apiKey
//          name: access_token
//          in: query
//     AuthorizationHeaderToken:
//          type: apiKey
//          name: Authorization
//          in: header
//          description: API tokens must be prepended with "token" followed by a space.
//     SudoParam:
//          type: apiKey
//          name: sudo
//          in: query
//          description: Sudo API request as the user provided as the key. Admin privileges are required.
//     SudoHeader:
//          type: apiKey
//          name: Sudo
//          in: header
//          description: Sudo API request as the user provided as the key. Admin privileges are required.
//
// swagger:meta
package catalog

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	v4 "code.gitea.io/gitea/routers/api/catalog/v4"
	v5 "code.gitea.io/gitea/routers/api/catalog/v5"
	"code.gitea.io/gitea/routers/api/v1/misc"
	_ "code.gitea.io/gitea/routers/api/v1/swagger" // for swagger generation
	"fmt"
	"net/http"

	"gitea.com/macaron/macaron"
)

var Versions = []string{
	"v4",
	"v5",
}
var LatestVersion = Versions[len(Versions)-1]

func sudo() macaron.Handler {
	return func(ctx *context.APIContext) {
		sudo := ctx.Query("sudo")
		if len(sudo) == 0 {
			sudo = ctx.Req.Header.Get("Sudo")
		}

		if len(sudo) > 0 {
			if ctx.IsSigned && ctx.User.IsAdmin {
				user, err := models.GetUserByName(sudo)
				if err != nil {
					if models.IsErrUserNotExist(err) {
						ctx.NotFound()
					} else {
						ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
					}
					return
				}
				log.Trace("Sudo from (%s) to: %s", ctx.User.Name, user.Name)
				ctx.User = user
			} else {
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "Only administrators allowed to sudo.",
				})
				return
			}
		}
	}
}

// RegisterRoutes registers all Catalog v4 APIs routes to web application.
// FIXME: custom form error response
func RegisterRoutes(m *macaron.Macaron) {
	if setting.API.EnableSwagger {
		m.Get("/swagger", misc.Swagger) // Render catalog by default
	}

	m.Group("", func() {
		m.Get("", ListCatalogVersionEndpoints)

		m.Group("/latest", func() {
			m.Get("", func(ctx *context.APIContext) {
				ctx.Redirect(fmt.Sprintf("/api/catalog/%s", LatestVersion))
			})
			m.Get("/*", func(ctx *context.APIContext) {
				ctx.Redirect(fmt.Sprintf("/api/catalog/%s/%s", LatestVersion, ctx.Params("*")))
			})
		})

		v4.RegisterRoutes(m)
		v5.RegisterRoutes(m)
	}, securityHeaders(), context.APIContexter(), sudo())
}

func securityHeaders() macaron.Handler {
	return func(ctx *macaron.Context) {
		ctx.Resp.Before(func(w macaron.ResponseWriter) {
			// CORB: https://www.chromium.org/Home/chromium-security/corb-for-developers
			// http://stackoverflow.com/a/3146618/244009
			w.Header().Set("x-content-type-options", "nosniff")
		})
	}
}
