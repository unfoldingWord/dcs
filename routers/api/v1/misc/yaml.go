// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/yaml"
)

type YamlOption struct {
	Text string
}

// https://github.com/gogits/go-gogs-client/wiki/Miscellaneous#render-an-arbitrary-markdown-document
func Yaml(ctx *context.APIContext, form YamlOption) {
	if ctx.HasAPIError() {
		ctx.Error(422, "", ctx.GetErrMsg())
		return
	}

	if len(form.Text) == 0 {
		ctx.Write([]byte(""))
		return
	}
	if rendered, err := yaml.RenderSanitized([]byte(form.Text)); err != nil {
		ctx.Error(400, "Unable to parse YAML", err)
	} else {
		ctx.Write(rendered)
	}
}
