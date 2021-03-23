package catalog

import (
	"code.gitea.io/gitea/modules/structs"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// ListCatalogEndpoints Lists all the Catalog Endpoints for all versions
func ListCatalogVersionEndpoints(ctx *context.APIContext) {
	// swagger:operation GET /misc/versions catalog catalogListCatalogVersionEndpoints
	// ---
	// summary: Catalog version endpoint list, including what version "latest points to
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogVersionEndpointsResponse"
	//   "422":
	//     "$ref": "#/responses/validationError"

	versionEndpoints := structs.CatalogVersionEndpoints{
		Latest:   LatestVersion,
		Versions: map[string]string{},
	}

	for _, version := range Versions {
		versionEndpoints.Versions[version] = fmt.Sprintf("%sapi/catalog/%s", setting.AppURL, version)
	}

	resp := map[string]interface{}{
		"ok":   true,
		"data": versionEndpoints,
	}
	ctx.JSON(http.StatusOK, resp)
}
