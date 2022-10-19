// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package catalog

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"

	"github.com/go-chi/cors"
)

// Routes registers all v1 APIs routes to web application.
func Routes() *web.Route {
	m := web.NewRoute()

	if setting.CORSConfig.Enabled {
		m.Use(cors.Handler(cors.Options{
			// Scheme:           setting.CORSConfig.Scheme, // FIXME: the cors middleware needs scheme option
			AllowedOrigins: setting.CORSConfig.AllowDomain,
			// setting.CORSConfig.AllowSubdomain // FIXME: the cors middleware needs allowSubdomain option
			AllowedMethods:   setting.CORSConfig.Methods,
			AllowCredentials: setting.CORSConfig.AllowCredentials,
			AllowedHeaders:   []string{"Authorization", "X-Gitea-OTP"},
			MaxAge:           int(setting.CORSConfig.MaxAge.Seconds()),
		}))
	}
	m.Use(context.APIContexter())

	m.Use(context.ToggleAPI(&context.ToggleOptions{
		SignInRequired: setting.Service.RequireSignInView,
	}))

	m.Group("", func() {
		m.Get("/swagger", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL+"/api/swagger#/catalog", http.StatusMovedPermanently)
		})
		m.Group("/v5", func() {
			m.Get("", func(ctx *context.APIContext) {
				var query string
				if ctx.Req.URL.RawQuery != "" {
					query = "?" + ctx.Req.URL.RawQuery
				}
				ctx.Redirect(fmt.Sprintf("/api/v1/catalog/%s", query), http.StatusPermanentRedirect)
			})
			m.Get("/*", func(ctx *context.APIContext) {
				var query string
				if ctx.Req.URL.RawQuery != "" {
					query = "?" + ctx.Req.URL.RawQuery
				}
				ctx.Redirect(fmt.Sprintf("/api/v1/catalog/%s%s", ctx.Params("*"), query), http.StatusPermanentRedirect)
			})
		})
	})

	return m
}
