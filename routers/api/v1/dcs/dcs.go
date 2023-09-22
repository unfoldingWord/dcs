// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"net/http"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/util"
)

// ServeLangnamesJSON serves the langname.json file from td.unfoldingword.org
func ServeLangnamesJSON(ctx *context.APIContext) {
	// swagger:operation GET /languages/langnames.json languages languagesLangnamesJSON
	// ---
	// summary: Fetches the langnames.json file
	// produces:
	// - application/json
	// parameters:
	// - name: lc
	//   in: query
	//   description: list only the given language codes
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: gw
	//   in: query
	//   description: if true, only show gateway languages, if false, only non-gateway languages (other languages)
	//   type: boolean
	// - name: ld
	//   in: query
	//   description: direction of the language
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [ltr,rtl]
	// - name: sort
	//   in: query
	//   description: sort alphanumerically by attribute. Supported values are
	//                "ang", "hc", "lc", "ld", "ln", "lr".
	//                Default is "lc"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc"
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/LangnamesJSON"

	ctx.JSON(http.StatusOK, searchLangnamesJSON(ctx))
}

// ServeLangnamesJSONKeyed serves the langname.json file from td.unfoldingword.org but keyed by lang code
func ServeLangnamesJSONKeyed(ctx *context.APIContext) {
	// swagger:operation GET /languages/langnames_keyed.json languages languagesLangnamesJSONKeyed
	// ---
	// summary: Fetches the langnames.json file
	// produces:
	// - application/json
	// parameters:
	// - name: lc
	//   in: query
	//   description: list only the given language codes
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: gw
	//   in: query
	//   description: if true, only show gateway languages, if false, only non-gateway languages (other languages)
	//   type: boolean
	// - name: ld
	//   in: query
	//   description: direction of the language
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [ltr,rtl]
	// responses:
	//   "200":
	//     "$ref": "#/responses/LangnamesJSON"

	ctx.JSON(http.StatusOK, searchLangnamesJSONKeyed(ctx))
}

func searchLangnamesJSON(ctx *context.APIContext) []map[string]interface{} {
	langnames := dcs.GetLangnamesJSON()
	if len(langnames) == 0 {
		return langnames
	}

	lcArr := ctx.FormStrings("lc")
	gw := ctx.FormOptionalBool("gw")
	ld := ctx.FormString("ld")
	sortField := ctx.FormString("sort")
	if sortField != "ang" && sortField != "hc" && sortField != "lc" && sortField != "ld" && sortField != "ln" && sortField != "lr" {
		sortField = "lc"
	}
	orderAsc := true
	if strings.ToLower(ctx.FormString("order")) == "desc" {
		orderAsc = false
	}

	if len(lcArr) > 0 || gw != util.OptionalBoolNone || ld != "" {
		filteredLangnames := []map[string]interface{}{}
		for _, data := range langnames {
			lcMatches := true
			gwMatches := true
			ldMatches := true

			if len(lcArr) > 0 {
				lcMatches = false
				for _, lc := range lcArr {
					if data["lc"] == lc {
						lcMatches = true
						break
					}
				}
			}
			if gw != util.OptionalBoolNone {
				gwMatches = false
				if data["gw"].(bool) == gw.IsTrue() {
					gwMatches = true
				}
			}
			if ld != "" {
				ldMatches = false
				if data["ld"] == ld {
					ldMatches = true
				}
			}
			if lcMatches && gwMatches && ldMatches {
				filteredLangnames = append(filteredLangnames, data)
			}
		}
		langnames = filteredLangnames
	}

	sort.Slice(langnames, func(i, j int) bool {
		iStr, ok := langnames[i][sortField].(string)
		if !ok {
			return true
		}
		jStr, ok := langnames[j][sortField].(string)
		if !ok {
			return false
		}
		if iStr == jStr {
			iStr = langnames[i]["lc"].(string)
			jStr = langnames[j]["lc"].(string)
		}
		if orderAsc {
			return iStr < jStr
		}
		return iStr > jStr
	})

	return langnames
}

func searchLangnamesJSONKeyed(ctx *context.APIContext) map[string]map[string]interface{} {
	langnames := searchLangnamesJSON(ctx)
	langnamesKeyed := map[string]map[string]interface{}{}
	for _, data := range langnames {
		langnamesKeyed[data["lc"].(string)] = data
	}
	return langnamesKeyed
}
