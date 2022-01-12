// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v3

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	v5 "code.gitea.io/gitea/routers/api/catalog/v5"
)

// CatalogV3 catalog v3 listing, back-port of https://api.door43.org/v3/catalog.json
func CatalogV3(ctx *context.APIContext) {
	// swagger:operation GET /v3/catalog.json v3 CatalogV3
	// ---
	// summary: Catalog v3 listing by language, back-port of https://api.door43.org/v3/catalog.json
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResultsV3"
	//   "422":
	//     "$ref": "#/responses/validationError"

	searchCatalog(ctx)
}

// CatalogSearchV3 catalog v3 search
func CatalogSearchV3(ctx *context.APIContext) {
	// swagger:operation GET /v3/search v3 CatalogSearchV3
	// ---
	// summary: Catalog v3 search
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: query
	//   description: search only for entries with the given owner name(s). Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: repo
	//   in: query
	//   description: search only for entries with the given repo name(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: lang
	//   in: query
	//   description: search only for entries with the given language(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, subject, owner and repo search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResultsV3"
	//   "422":
	//     "$ref": "#/responses/validationError"

	searchCatalog(ctx)
}

func searchCatalog(ctx *context.APIContext) {
	listOptions := db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: ctx.FormInt("limit"),
	}
	if listOptions.Page < 1 {
		listOptions.Page = 1
	}

	subjectQueryStrings := v5.QueryStrings(ctx, "subject")
	for i, subject := range subjectQueryStrings {
		subjectQueryStrings[i] = strings.ReplaceAll(subject, "_", " ")
	}

	opts := &models.SearchCatalogOptions{
		ListOptions:  listOptions,
		Owners:       v5.QueryStrings(ctx, "owner"),
		Repos:        v5.QueryStrings(ctx, "repo"),
		Languages:    v5.QueryStrings(ctx, "lang"),
		Subjects:     subjectQueryStrings,
		PartialMatch: ctx.FormBool("partialMatch"),
	}

	prodDMs, err := models.QueryForCatalogV3(opts)
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
	for _, dm := range prodDMs {
		langInfo := (*dm.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})
		if currentLang == nil || currentLang.Identifier != langInfo["identifier"].(string) {
			currentLang = &api.CatalogV3Language{
				Identifier: langInfo["identifier"].(string),
				Title:      langInfo["title"].(string),
				Direction:  langInfo["direction"].(string),
			}
			languages = append(languages, currentLang)
		}
		currentLang.Resources = append(currentLang.Resources, convert.ToCatalogV3Resource(dm))
		if dm.ReleaseDateUnix > allLastUpdated {
			allLastUpdated = dm.ReleaseDateUnix
		}
		if dm.ReleaseDateUnix > langLastUpdated {
			langLastUpdated = dm.ReleaseDateUnix
			currentLang.LastUpdated = langLastUpdated.AsTime()
		}
		currentLang.Resources = append(currentLang.Resources, convert.ToCatalogV3Resource(dm))
	}

	ctx.JSON(http.StatusOK, api.CatalogV3{
		Languages: languages,
	})
}

// CatalogSubjectsPivotedV3 catalog v3 listing pivoted by subject/language, back-port of https://api.door43.org/v3/subjects/pivoted.json
func CatalogSubjectsPivotedV3(ctx *context.APIContext) {
	// swagger:operation GET /v3/subjects/pivoted.json v3 CatalogSubjectsPivotedV3
	// ---
	// summary: Catalog v3 listing pivoted by subject/language, back-port of https://api.door43.org/v3/subjects/pivoted.json
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResultsPivotedV3"
	//   "422":
	//     "$ref": "#/responses/validationError"
	searchCatalogPivoted(ctx)
}

// CatalogSubjectsPivotedSearchV3 catalog v3 search that pivotes the catalog by subject/language
func CatalogSubjectsPivotedSearchV3(ctx *context.APIContext) {
	// swagger:operation GET /v3/subjects/search v3 CatalogSubjectsPivotedSearchV3
	// ---
	// summary: Catalog v3 search pivoted by subject/language
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: query
	//   description: search only for entries with the given owner name(s). Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: repo
	//   in: query
	//   description: search only for entries with the given repo name(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: lang
	//   in: query
	//   description: search only for entries with the given language(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, subject, owner and repo search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResultsPivotedV3"
	//   "422":
	//     "$ref": "#/responses/validationError"

	searchCatalogPivoted(ctx)
}

// CatalogSubjectsPivotedBySubjectV3 catalog v3 listing that pivotes the catalog by subject/language for a single subject
func CatalogSubjectsPivotedBySubjectV3(ctx *context.APIContext) {
	// swagger:operation GET /v3/subjects/{subject}.json v3 CatalogSubjectsPivotedBySubjectV3
	// ---
	// summary: Catalog v3 listing pivoted on subject by a given subject (e.g. /v3/subjects/Open_Bible_Stories.json)
	// produces:
	// - application/json
	// parameters:
	// - name: subject
	//   in: path
	//   description: subject to query
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResultsPivotedV3"
	//   "422":
	//     "$ref": "#/responses/validationError"
	searchCatalogPivoted(ctx)
}

func searchCatalogPivoted(ctx *context.APIContext) {
	listOptions := db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: ctx.FormInt("limit"),
	}
	if listOptions.Page < 1 {
		listOptions.Page = 1
	}

	var subjectQueryStrings []string
	if ctx.Params("subject") != "" {
		subjectQueryStrings = []string{ctx.Params("subject")}
	} else {
		subjectQueryStrings = v5.QueryStrings(ctx, "subject")
	}
	for i, subject := range subjectQueryStrings {
		subjectQueryStrings[i] = strings.ReplaceAll(subject, "_", " ")
	}

	opts := &models.SearchCatalogOptions{
		ListOptions:  listOptions,
		Owners:       v5.QueryStrings(ctx, "owner"),
		Repos:        v5.QueryStrings(ctx, "repo"),
		Languages:    v5.QueryStrings(ctx, "lang"),
		Subjects:     subjectQueryStrings,
		PartialMatch: ctx.FormBool("partialMatch"),
		OrderBy:      []models.CatalogOrderBy{models.CatalogOrderBySubject},
	}

	prodDMs, err := models.QueryForCatalogV3(opts)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	subjects := []*api.CatalogV3Subject{}
	var currentSubject *api.CatalogV3Subject
	var allLastUpdated timeutil.TimeStamp
	var subjectLastUpdated timeutil.TimeStamp
	for _, dm := range prodDMs {
		core := (*dm.Metadata)["dublin_core"].(map[string]interface{})
		langInfo := core["language"].(map[string]interface{})
		if currentSubject == nil || currentSubject.Identifier != langInfo["identifier"].(string) || currentSubject.Subject != core["subject"].(string) {
			subjectLastUpdated = 0
			currentSubject = &api.CatalogV3Subject{
				Subject:    strings.ReplaceAll(core["subject"].(string), " ", "_"),
				Identifier: strings.ReplaceAll(core["subject"].(string), " ", "_"),
				Language:   langInfo["identifier"].(string),
				Direction:  langInfo["direction"].(string),
				Title:      langInfo["title"].(string),
			}
			subjects = append(subjects, currentSubject)
		}
		currentSubject.Resources = append(currentSubject.Resources, convert.ToCatalogV3Resource(dm))
		if dm.ReleaseDateUnix > allLastUpdated {
			allLastUpdated = dm.ReleaseDateUnix
		}
		if dm.ReleaseDateUnix > subjectLastUpdated {
			subjectLastUpdated = dm.ReleaseDateUnix
			currentSubject.LastUpdated = subjectLastUpdated.AsTime()
		}
	}

	ctx.JSON(http.StatusOK, api.CatalogV3Pivoted{
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
		Subjects:    subjects,
		LastUpdated: allLastUpdated.AsTime(),
	})
}
