// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v3

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
)

var searchOrderByMap = map[string]map[string]models.CatalogOrderBy{
	"asc": {
		"subject":  models.CatalogOrderBySubject,
		"title":    models.CatalogOrderByTitle,
		"released": models.CatalogOrderByOldest,
		"lang":     models.CatalogOrderByLangCode,
		"releases": models.CatalogOrderByReleases,
		"stars":    models.CatalogOrderByStars,
		"forks":    models.CatalogOrderByForks,
		"tag":      models.CatalogOrderByTag,
	},
	"desc": {
		"title":    models.CatalogOrderByTitleReverse,
		"subject":  models.CatalogOrderBySubjectReverse,
		"released": models.CatalogOrderByNewest,
		"lang":     models.CatalogOrderByLangCodeReverse,
		"releases": models.CatalogOrderByReleasesReverse,
		"stars":    models.CatalogOrderByStarsReverse,
		"forks":    models.CatalogOrderByForksReverse,
		"tag":      models.CatalogOrderByTagReverse,
	},
}

// CatalogV3 search the catalog via options
func CatalogV3(ctx *context.APIContext) {
	// swagger:operation GET /v3 v3 CatalogV3
	// ---
	// summary: Catalog v3 listing
	// produces:
	// - application/json
	// parameters:
	// - name: subject
	//   in: query
	//   description: If present, should list only items by the given subject (Bible, Aligned Bible, Open Bible Stories, etc.)
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResultsV5"
	//   "422":
	//     "$ref": "#/responses/validationError"

	subject := ctx.Query("subject", "")

	prodDMs, err := models.QueryForCatalogV3(subject)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	languages := []*api.CatalogV3Language{}
	var currentLang *api.CatalogV3Language
	for _, dm := range prodDMs {
		langInfo := (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})
		if currentLang == nil || currentLang.Identifier != langInfo["identifier"].(string) {
			currentLang = &api.CatalogV3Language{
				Identifier: langInfo["identifier"].(string),
				Title:      langInfo["title"].(string),
				Direction:  langInfo["direction"].(string),
			}
			currentLang.Resources = append(currentLang.Resources, convert.ToCatalogV3Resource(dm))
		}
	}

	ctx.JSON(http.StatusOK, api.CatalogV3{
		Languages: languages,
	})
}
