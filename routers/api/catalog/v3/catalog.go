// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v3

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
)

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
	fmt.Printf("\n\nSTART!!!\n\n")
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
	var allLastUpdated timeutil.TimeStamp
	var langLastUpdated timeutil.TimeStamp
	fmt.Printf("lastUpdated: %d, %s\n", allLastUpdated, allLastUpdated.AsTime())
	for _, dm := range prodDMs {
		langInfo := (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})
		if currentLang == nil || currentLang.Identifier != langInfo["identifier"].(string) {
			langLastUpdated = 0
			currentLang = &api.CatalogV3Language{
				Identifier: langInfo["identifier"].(string),
				Title:      langInfo["title"].(string),
				Direction:  langInfo["direction"].(string),
			}
			languages = append(languages, currentLang)
		}
		currentLang.Resources = append(currentLang.Resources, convert.ToCatalogV3Resource(dm))
		fmt.Printf("%d, %s\n", dm.ReleaseDateUnix, dm.ReleaseDateUnix.AsTime())
		if dm.ReleaseDateUnix > allLastUpdated {
			fmt.Printf("%d => %d, %s => %s", allLastUpdated, dm.ReleaseDateUnix, allLastUpdated.AsTime(), dm.ReleaseDateUnix.AsTime())
			allLastUpdated = dm.ReleaseDateUnix
		}
		if dm.ReleaseDateUnix > langLastUpdated {
			langLastUpdated = dm.ReleaseDateUnix
			currentLang.LastUpdated = langLastUpdated.AsTime()
		}
	}

	ctx.JSON(http.StatusOK, api.CatalogV3{
		Catalogs: []map[string]string{
			{
				"identifier": "langnames",
				"modified":   "2016-10-03",
				"url":        "https://td.unfoldingword.org/exports/langnames.json",
			},
			{
				"identifier": "temp-langnames",
				"modified":   "2016-10-03",
				"url":        "https://td.unfoldingword.org/api/templanguages/",
			},
			{
				"identifier": "approved-temp-langnames",
				"modified":   "2016-10-03",
				"url":        "https://td.unfoldingword.org/api/templanguages/assignment/changed/",
			},
			{
				"identifier": "new-language-questions",
				"modified":   "2016-10-03",
				"url":        "https://td.unfoldingword.org/api/questionnaire/",
			},
		},
		Languages:   languages,
		LastUpdated: allLastUpdated.AsTime(),
	})
}
