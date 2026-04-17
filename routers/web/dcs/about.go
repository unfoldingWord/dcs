// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

/*** DCS Customizations - Router for About page ***/

package dcs

import (
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const (
	// tplAbout about page template. This is the same as the home page that
	// unauthenticated users see.
	tplAbout templates.TplName = "home"
	tplTools templates.TplName = "tools"
)

// About render about page
func About(ctx *context.Context) {
	ctx.Data["PageIsAbout"] = true
	ctx.HTML(200, tplAbout)
}

// Tools render tools page
func Tools(ctx *context.Context) {
	ctx.Data["PageIsTools"] = true
	ctx.HTML(200, tplTools)
}
