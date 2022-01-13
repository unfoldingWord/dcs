// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package catalog

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

// tplSwagger swagger page template
const tplCatalogSwagger base.TplName = "swagger/dcs/catalog_ui"
const tplDcsSwagger base.TplName = "swagger/dcs/dcs_ui"

// CatalogSwagger render swagger-ui page for the Catalog API
func CatalogSwagger(ctx *context.Context) {
	ctx.Data["APIJSONVersion"] = "catalog"
	ctx.HTML(http.StatusOK, tplCatalogSwagger)
}

// DcsSwagger render swagger-ui page for the whole DCS
func DcsSwagger(ctx *context.Context) {
	ctx.Data["APIJSONVersion"] = "dcs"
	ctx.HTML(http.StatusOK, tplDcsSwagger)
}
