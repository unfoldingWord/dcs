// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package catalog

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
)

var searchOrderByMap = map[string]map[string]door43metadata.CatalogOrderBy{
	"asc": {
		"title":      door43metadata.CatalogOrderByTitle,
		"subject":    door43metadata.CatalogOrderBySubject,
		"identifier": door43metadata.CatalogOrderByIdentifier,
		"reponame":   door43metadata.CatalogOrderByRepoName,
		"released":   door43metadata.CatalogOrderByOldest,
		"lang":       door43metadata.CatalogOrderByLangCode,
		"releases":   door43metadata.CatalogOrderByReleases,
		"stars":      door43metadata.CatalogOrderByStars,
		"forks":      door43metadata.CatalogOrderByForks,
		"tag":        door43metadata.CatalogOrderByTag,
	},
	"desc": {
		"title":      door43metadata.CatalogOrderByTitleReverse,
		"subject":    door43metadata.CatalogOrderBySubjectReverse,
		"identifier": door43metadata.CatalogOrderByIdentifierReverse,
		"reponame":   door43metadata.CatalogOrderByRepoNameReverse,
		"released":   door43metadata.CatalogOrderByNewest,
		"lang":       door43metadata.CatalogOrderByLangCodeReverse,
		"releases":   door43metadata.CatalogOrderByReleasesReverse,
		"stars":      door43metadata.CatalogOrderByStarsReverse,
		"forks":      door43metadata.CatalogOrderByForksReverse,
		"tag":        door43metadata.CatalogOrderByTagReverse,
	},
}

// Search search the catalog via options
func Search(ctx *context.APIContext) {
	// swagger:operation GET /catalog/search catalog catalogSearch
	// ---
	// summary: Catalog search
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: keyword(s). Can use multiple `q=<keyword>`s or a comma-delimited string for more than one keyword. Is case insensitive
	//   type: string
	// - name: owner
	//   in: query
	//   description: search only for entries with the given owner name(s). Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: repo
	//   in: query
	//   description: search only for entries with the given repo name(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: tag
	//   in: query
	//   description: search only for entries with the given release tag(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: lang
	//   in: query
	//   description: search only for entries with the given language(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: stage
	//   in: query
	//   description: 'specifies which release stage to be return of these stages:
	//                "prod" - return only the production releases (default);
	//                "preprod" - return the pre-production release if it exists instead of the production release;
	//                "draft" - return the draft release if it exists instead of pre-production or production release;
	//                "latest" -return the default branch (e.g. master) if it is a valid RC instead of the above'
	//   type: string
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (project ids). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, subject, owner and repo search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// - name: includeHistory
	//   in: query
	//   description: if true, all releases, not just the latest, are included. Default is false
	//   type: boolean
	// - name: includeMetadata
	//   in: query
	//   description: if false, only subject and title are searched with query terms, if true all metadata values are searched. Default is true
	//   type: boolean
	// - name: showIngredients
	//   in: query
	//   description: if true, a list of the projects in the resource and their file paths will be listed for each entry. Default is false
	//   type: boolean
	// - name: sort
	//   in: query
	//   description: sort repos alphanumerically by attribute. Supported values are
	//                "subject", "title", "reponame", "tag", "released", "lang", "releases", "stars", "forks".
	//                Default is by "language", "subject" and then "tag"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, defaults to no limit
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResults"
	//   "422":
	//     "$ref": "#/responses/validationError"

	searchCatalog(ctx)
}

// SearchOwner search the catalog via owner and via options
func SearchOwner(ctx *context.APIContext) {
	// swagger:operation GET /catalog/search/{owner} catalog catalogSearchOwner
	// ---
	// summary: Catalog search by owner
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the returned entries
	//   type: string
	//   required: true
	// - name: q
	//   in: query
	//   description: keyword(s). Can use multiple `q=<keyword>`s or a comma-delimited string for more than one keyword. Is case insensitive
	//   type: string
	// - name: repo
	//   in: query
	//   description: search only for entries with the given repo name(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: tag
	//   in: query
	//   description: search only for entries with the given release tag(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: lang
	//   in: query
	//   description: search only for entries with the given language(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: stage
	//   in: query
	//   description: 'specifies which release stage to be return of these stages:
	//                "prod" - return only the production releases (default);
	//                "preprod" - return the pre-production release if it exists instead of the production release;
	//                "draft" - return the draft release if it exists instead of pre-production or production release;
	//                "latest" -return the default branch (e.g. master) if it is a valid RC instead of the above'
	//   type: string
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (project ids). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, subject, owner and repo search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// - name: includeHistory
	//   in: query
	//   description: if true, all releases, not just the latest, are included. Default is false
	//   type: boolean
	// - name: includeMetadata
	//   in: query
	//   description: if false, only subject and title are searched with query terms, if true all metadata values are searched. Default is true
	//   type: boolean
	// - name: showIngredients
	//   in: query
	//   description: if true, a list of the projects in the resource and their file paths will be listed for each entry. Default is false
	//   type: boolean
	// - name: sort
	//   in: query
	//   description: sort repos alphanumerically by attribute. Supported values are
	//                "subject", "title", "reponame", "tag", "released", "lang", "releases", "stars", "forks".
	//                Default is by "language", "subject" and then "tag"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, defaults to no limit
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResults"
	//   "422":
	//     "$ref": "#/responses/validationError"

	searchCatalog(ctx)
}

// SearchRepo search the catalog via repo and options
func SearchRepo(ctx *context.APIContext) {
	// swagger:operation GET /catalog/search/{owner}/{repo} catalog catalogSearchRepo
	// ---
	// summary: Catalog search by repo
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the returned entries
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo of the returned entries
	//   type: string
	//   required: true
	// - name: q
	//   in: query
	//   description: keyword(s). Can use multiple `q=<keyword>`s or a comma-delimited string for more than one keyword. Is case insensitive
	//   type: string
	// - name: owner
	//   in: query
	//   description: search only for entries with the given owner name(s). Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: repo
	//   in: query
	//   description: search only for entries with the given repo name(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: tag
	//   in: query
	//   description: search only for entries with the given release tag(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: lang
	//   in: query
	//   description: search only for entries with the given language(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: stage
	//   in: query
	//   description: 'specifies which release stage to be return of these stages:
	//                "prod" - return only the production releases (default);
	//                "preprod" - return the pre-production release if it exists instead of the production release;
	//                "draft" - return the draft release if it exists instead of pre-production or production release;
	//                "latest" -return the default branch (e.g. master) if it is a valid RC instead of the above'
	//   type: string
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (project ids). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, subject, owner and repo search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// - name: includeHistory
	//   in: query
	//   description: if true, all releases, not just the latest, are included. Default is false
	//   type: boolean
	// - name: includeMetadata
	//   in: query
	//   description: if false, only subject and title are searched with query terms, if true all metadata values are searched. Default is true
	//   type: boolean
	// - name: showIngredients
	//   in: query
	//   description: if true, a list of the projects in the resource and their file paths will be listed for each entry. Default is false
	//   type: boolean
	// - name: sort
	//   in: query
	//   description: sort repos alphanumerically by attribute. Supported values are
	//                "subject", "title", "reponame", "tag", "released", "lang", "releases", "stars", "forks".
	//                Default is by "language", "subject" and then "tag"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, defaults to no limit
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogSearchResults"
	//   "422":
	//     "$ref": "#/responses/validationError"

	searchCatalog(ctx)
}

// GetCatalogEntry Get the catalog entry from the given ownername, reponame and ref
func GetCatalogEntry(ctx *context.APIContext) {
	// swagger:operation GET /catalog/entry/{owner}/{repo}/{tag} catalog catalogGetEntry
	// ---
	// summary: Catalog entry
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: tag
	//   in: path
	//   description: release tag or default branch
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogEntry"
	//   "422":
	//     "$ref": "#/responses/validationError"

	tag := ctx.Params("tag")
	var dm *repo.Door43Metadata
	var err error
	if tag == ctx.Repo.Repository.DefaultBranch {
		dm, err = repo.GetDoor43MetadataByRepoIDAndReleaseID(ctx.Repo.Repository.ID, 0)
	} else {
		dm, err = repo.GetDoor43MetadataByRepoIDAndTagName(ctx.Repo.Repository.ID, tag)
	}
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}
	if err := dm.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}
	accessMode, err := access_model.AccessLevel(ctx, ctx.ContextUser, dm.Repo)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
	}
	ctx.JSON(http.StatusOK, convert.ToCatalogEntry(dm, accessMode))
}

// GetCatalogMetadata Get the metadata (RC 0.2.0 manifest) in JSON format for the given ownername, reponame and ref
func GetCatalogMetadata(ctx *context.APIContext) {
	// swagger:operation GET /catalog/entry/{owner}/{repo}/{tag}/metadata catalog catalogGetMetadata
	// ---
	// summary: Catalog entry metadata (manifest.yaml in JSON format)
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: tag
	//   in: path
	//   description: release tag or default branch
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogMetadata"
	//   "422":
	//     "$ref": "#/responses/validationError"

	dm, err := repo.GetDoor43MetadataByRepoIDAndTagName(ctx.Repo.Repository.ID, ctx.Repo.TagName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}
	ctx.JSON(http.StatusOK, dm.Metadata)
}

// QueryStrings After calling QueryStrings on the context, it also separates strings that have commas into substrings
func QueryStrings(ctx *context.APIContext, name string) []string {
	strs := ctx.FormStrings(name)
	if len(strs) == 0 {
		return strs
	}
	var newStrs []string
	for _, str := range strs {
		newStrs = append(newStrs, door43metadata.SplitAtCommaNotInString(str, false)...)
	}
	return newStrs
}

func searchCatalog(ctx *context.APIContext) {
	var repoID int64
	var owners, repos []string
	includeMetadata := true
	if ctx.Repo.Repository != nil {
		repoID = ctx.Repo.Repository.ID
	} else {
		if ctx.Params("username") != "" {
			owners = []string{ctx.Params("username")}
		} else {
			owners = QueryStrings(ctx, "owner")
		}
		repos = QueryStrings(ctx, "repo")
	}
	if ctx.FormString("includeMetadata") != "" {
		includeMetadata = ctx.FormBool("includeMetadata")
	}

	stageStr := ctx.FormString("stage")
	var stage door43metadata.Stage
	if stageStr != "" {
		var ok bool
		stage, ok = door43metadata.StageMap[stageStr]
		if !ok {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("invalid stage: \"%s\"", stageStr))
			return
		}
	}

	keywords := []string{}
	query := strings.Trim(ctx.FormString("q"), " ")
	if query != "" {
		keywords = door43metadata.SplitAtCommaNotInString(query, false)
	}
	listOptions := db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: ctx.FormInt("limit"),
	}
	if listOptions.Page < 1 {
		listOptions.Page = 1
	}

	opts := &door43metadata.SearchCatalogOptions{
		ListOptions:     listOptions,
		Keywords:        keywords,
		Owners:          owners,
		Repos:           repos,
		RepoID:          repoID,
		Tags:            QueryStrings(ctx, "tag"),
		Stage:           stage,
		Languages:       QueryStrings(ctx, "lang"),
		Subjects:        QueryStrings(ctx, "subject"),
		CheckingLevels:  QueryStrings(ctx, "checkingLevel"),
		Books:           QueryStrings(ctx, "book"),
		IncludeHistory:  ctx.FormBool("includeHistory"),
		ShowIngredients: ctx.FormBool("showIngredients"),
		IncludeMetadata: includeMetadata,
		PartialMatch:    ctx.FormBool("partialMatch"),
	}

	sortModes := QueryStrings(ctx, "sort")
	if len(sortModes) > 0 {
		sortOrder := ctx.FormString("order")
		if sortOrder == "" {
			sortOrder = "asc"
		}
		if searchModeMap, ok := searchOrderByMap[sortOrder]; ok {
			for _, sortMode := range sortModes {
				if orderBy, ok := searchModeMap[strings.ToLower(sortMode)]; ok {
					opts.OrderBy = append(opts.OrderBy, orderBy)
				} else {
					ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("invalid sort mode: \"%s\"", sortMode))
					return
				}
			}
		} else {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("invalid sort order: \"%s\"", sortOrder))
			return
		}
	} else {
		opts.OrderBy = []door43metadata.CatalogOrderBy{door43metadata.CatalogOrderByLangCode, door43metadata.CatalogOrderBySubject, door43metadata.CatalogOrderByReleaseDateReverse}
	}

	dms, count, err := models.SearchCatalog(opts)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	results := make([]*api.CatalogEntry, len(dms))
	var lastUpdated time.Time
	for i, dm := range dms {
		accessMode, err := access_model.AccessLevel(ctx, ctx.ContextUser, dm.Repo)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, api.SearchError{
				OK:    false,
				Error: err.Error(),
			})
		}
		dmAPI := convert.ToCatalogEntry(dm, accessMode)
		if !opts.ShowIngredients {
			dmAPI.Ingredients = nil
		}
		if dmAPI.Released.After(lastUpdated) {
			lastUpdated = dmAPI.Released
		}
		results[i] = dmAPI
	}

	if lastUpdated.IsZero() {
		lastUpdated = time.Now()
	}

	if opts.PageSize > 0 {
		ctx.SetLinkHeader(int(count), opts.PageSize)
	} else {
		ctx.SetLinkHeader(int(count), int(count))
	}
	ctx.RespHeader().Set("X-Total-Count", fmt.Sprintf("%d", count))
	ctx.JSON(http.StatusOK, api.CatalogSearchResults{
		OK:          true,
		Data:        results,
		LastUpdated: lastUpdated,
	})
}
