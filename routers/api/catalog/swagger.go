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
const tplSwagger base.TplName = "swagger/catalog/ui"

// Swagger render swagger-ui page with v5 json
func Swagger(ctx *context.Context) {
	ctx.Data["APIJSONVersion"] = "v5"
	ctx.HTML(http.StatusOK, tplSwagger)
}
