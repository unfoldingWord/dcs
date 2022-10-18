// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package catalog

import (
	"net/http"
	"strings"

	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/catalog"

	"github.com/go-chi/cors"
)

func repoAssignment() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		userName := ctx.Params("username")
		repoName := ctx.Params("reponame")

		var (
			owner *user_model.User
			err   error
		)

		// Check if the user is the same as the repository owner.
		if ctx.IsSigned && ctx.Doer.LowerName == strings.ToLower(userName) {
			owner = ctx.Doer
		} else {
			owner, err = user_model.GetUserByName(ctx, userName)
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					if redirectUserID, err := user_model.LookupUserRedirect(userName); err == nil {
						context.RedirectToUser(ctx.Context, userName, redirectUserID)
					} else if user_model.IsErrUserRedirectNotExist(err) {
						ctx.NotFound("GetUserByName", err)
					} else {
						ctx.Error(http.StatusInternalServerError, "LookupUserRedirect", err)
					}
				} else {
					ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
				}
				return
			}
		}
		ctx.Repo.Owner = owner
		ctx.ContextUser = owner

		// Get repository.
		repo, err := repo_model.GetRepositoryByName(owner.ID, repoName)
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				redirectRepoID, err := repo_model.LookupRedirect(owner.ID, repoName)
				if err == nil {
					context.RedirectToRepo(ctx.Context, redirectRepoID)
				} else if repo_model.IsErrRedirectNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "LookupRepoRedirect", err)
				}
			} else {
				ctx.Error(http.StatusInternalServerError, "GetRepositoryByName", err)
			}
			return
		}

		repo.Owner = owner
		ctx.Repo.Repository = repo

		ctx.Repo.Permission, err = access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
			return
		}

		if !ctx.Repo.HasAccess() {
			ctx.NotFound()
			return
		}
	}
}

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
			ctx.Redirect(setting.AppSubURL + "/api/swagger#/catalog")
		})
		m.Group("/v5", func() {
			m.Get("", catalog.Search)
			m.Group("/search", func() {
				m.Get("", catalog.Search)
				m.Group("/{username}", func() {
					m.Get("", catalog.SearchOwner)
					m.Group("/{reponame}", func() {
						m.Get("", catalog.SearchRepo)
					})
				})
			})
			m.Group("/entry/{username}/{reponame}/{tag}", func() {
				m.Get("", catalog.GetCatalogEntry)
				m.Get("/metadata", catalog.GetCatalogMetadata)
			}, repoAssignment())
		})
	})

	return m
}

