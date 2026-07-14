// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

/*** DCS Customizations - Router for About page ***/

package dcs

import (
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
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
