// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

/*** DCS Customizations - Router for About page ***/

package dcs

import (
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	// tplAbout about page template. This is the same as the home page that
	// unauthenticated users see.
	tplAbout base.TplName = "home"
	tplTools base.TplName = "tools"
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
