package catalog

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// ListCatalogEndpoints Lists all the Catalog Endpoints for all versions
func ListCatalogVersionEndpoints(ctx *context.APIContext) {
	// swagger:operation GET /catalog catalog catalogListCatalogVersionEndpoints
	// ---
	// summary: Catalog version endpoint list, including "latest" pointing to the latest version
	// produces:
	// - application/json
	// parameters:
	// - name: version
	//   in: path
	//   description: version to list, all if empty
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogVersionList"
	//   "422":
	//     "$ref": "#/responses/validationError"

	versionInfo := map[string]interface{}{
		"latest": LatestVersion,
		"versions": map[string]string{},
	}

	for _, version := range Versions {
		versionInfo["versions"].(map[string]string)[version] = fmt.Sprintf("%sapi/catalog/%s", setting.AppURL, version)
	}

	resp := map[string]interface{}{
		"ok": true,
		"data": versionInfo,
	}
	ctx.JSON(http.StatusOK, resp)
}
