package ufw

import (
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	// tplAbout about page template. This is the same as the home page that
	// unauthenticated users see.
	tplAbout base.TplName = "home"
)

// About render about page
func About(ctx *context.Context) {
	ctx.Data["PageIsHome"] = true
	ctx.HTML(200, tplAbout)
}
