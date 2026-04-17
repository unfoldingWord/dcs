// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	"net/http"
	"strings" // DCS Customizations

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
)

/*** DCS Customizations ***/
// normalizeDCSVersion converts a DCS version like "1.25.7+dcs+2-g4f5ce5854b" into a form that
// hashicorp/go-version can parse ("1.25.7+dcs.2-g4f5ce5854b"). The issue is that the build
// metadata section only allows a single leading '+'; subsequent '+' signs (from git-describe
// output appended to the base tag) are invalid and cause version parsing to fail in remote
// clients that use the SDK's ServerVersion check (e.g. migration from another DCS instance).
//
// Strategy: replace every '+' after the first one with '.' so the build metadata remains a
// single dot-separated segment. For a clean tagged release ("1.25.7+dcs") nothing changes.
func normalizeDCSVersion(v string) string {
	firstPlus := strings.Index(v, "+")
	if firstPlus < 0 {
		return v
	}
	rest := v[firstPlus+1:]
	secondPlus := strings.Index(rest, "+")
	if secondPlus < 0 {
		return v // Only one '+' — already valid
	}
	// Replace all subsequent '+' with '.'
	normalized := v[:firstPlus+1+secondPlus] + "." + strings.ReplaceAll(rest[secondPlus+1:], "+", ".")
	return normalized
}

/*** END DCS Customizations ***/

// Version shows the version of the Gitea server
func Version(ctx *context.APIContext) {
	// swagger:operation GET /version miscellaneous getVersion
	// ---
	// summary: Returns the version of the Gitea application
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/ServerVersion"
	ctx.JSON(http.StatusOK, &structs.ServerVersion{Version: normalizeDCSVersion(setting.AppVer)}) // DCS Customizations
}
