// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package catalog

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

var searchOrderByMap = map[string]map[string]models.CatalogOrderBy{
	"asc": {
		"title":    models.CatalogOrderByTitle,
		"subject":  models.CatalogOrderBySubject,
		"created":  models.CatalogOrderByOldest,
		"lang":     models.CatalogOrderByLangCode,
		"releases": models.CatalogOrderByReleases,
		"stars":    models.CatalogOrderByStars,
		"forks":    models.CatalogOrderByForks,
	},
	"desc": {
		"title":    models.CatalogOrderByTitleReverse,
		"subject":  models.CatalogOrderBySubjectReverse,
		"created":  models.CatalogOrderByNewest,
		"lang":     models.CatalogOrderByLangCodeReverse,
		"releases": models.CatalogOrderByReleasesReverse,
		"stars":    models.CatalogOrderByStarsReverse,
		"forks":    models.CatalogOrderByForksReverse,
	},
}

// Search search the catalog via options
func Search(ctx *context.APIContext) {
	// swagger:operation GET /catalog catalog catalogSearch
	// ---
	// summary: Catalog search
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: keyword
	//   type: string
	// - name: topic
	//   in: query
	//   description: Limit search to repositories with keyword as topic
	//   type: boolean
	// - name: includeDesc
	//   in: query
	//   description: include search of keyword within repository description
	//   type: boolean
	// - name: stage
	//   in: query
	//   description: One ore more stages to return. Supported values are
	//                "prod" (production releases),
	//                "preprod" (pre-production releases),
	//                "draft" (draft releases), and
	//                "latest" (the default branch if it is a valid RC).
	//                Can have multiple stages given.
	//   type: string
	// - name: includeHistory
	//   in: query
	//   description: If true, all releases, not just the latest, are included
	//   type: bool
	// - name: searchAllMetadata
	//   in: query
	//   description: By default is true. If false, not all metadata values are searched, only subject and title
	//   type: bool
	// - name: lang
	//   in: query
	//   description: If the repo is a resource of the given language(s), the repo will be in the results. Multiple lang's are ORed.
	//   type: string
	// - name: subject
	//   in: query
	//   description: resource subject
	//   type: string
	// - name: book
	//   in: query
	//   description: book (project id) that exist in a resource. If the resource contains the
	//                the book, its repository will be included in the results
	//   type: string
	// - name: checking_level
	//   in: query
	//   description: Checking level of the resource can be 1, 2 or 3
	//   type: string
	// - name: sort
	//   in: query
	//   description: sort repos by attribute. Supported values are
	//                "alpha", "created", "updated", "size", and "id".
	//                Default is "alpha"
	//   type: string
	// - name: tag
	//   in: query
	//   description: A release tag that the catalog entry has to have. Useful with `subject` and `searchAllHistory` options.
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
	//   description: page size of results, maximum page size is 50
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
	// swagger:operation GET /catalog/{owner} catalog catalogSearchOwner
	// ---
	// summary: Catalog search by owner
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: keyword
	//   type: string
	// - name: topic
	//   in: query
	//   description: Limit search to repositories with keyword as topic
	//   type: boolean
	// - name: includeDesc
	//   in: query
	//   description: include search of keyword within repository description
	//   type: boolean
	// - name: owner
	//   in: query
	//   description: search only for repos with the given owner name.
	//   type: string
	// - name: repo
	//   in: query
	//   description: search only for repos with the given repo name.
	//   type: string
	// - name: ref
	//   in: query
	//   description: search only for catalog entries with the given ref (branch or tag)
	//   type: string
	// - name: stage
	//   in: query
	//   description: One ore more stages to return. Supported values are
	//                "prod" (production releases),
	//                "preprod" (pre-production releases),
	//                "draft" (draft releases), and
	//                "latest" (the default branch if it is a valid RC).
	//                Can have multiple stages given.
	//   type: string
	// - name: includeHistory
	//   in: query
	//   description: If true, all releases, not just the latest, are included
	//   type: bool
	// - name: searchAllMetadata
	//   in: query
	//   description: By default is true. If false, not all metadata values are searched, only subject and title
	//   type: bool
	// - name: lang
	//   in: query
	//   description: If the repo is a resource of the given language(s), the repo will be in the results. Multiple lang's are ORed.
	//   type: string
	// - name: subject
	//   in: query
	//   description: resource subject
	//   type: string
	// - name: book
	//   in: query
	//   description: book (project id) that exist in a resource. If the resource contains the
	//                the book, its repository will be included in the results
	//   type: string
	// - name: checking_level
	//   in: query
	//   description: Checking level of the resource can be 1, 2 or 3
	//   type: string
	// - name: sort
	//   in: query
	//   description: sort repos by attribute. Supported values are
	//                "alpha", "created", "updated", "size", and "id".
	//                Default is "alpha"
	//   type: string
	// - name: tag
	//   in: query
	//   description: A release tag that the catalog entry has to have. Useful with `subject` and `searchAllHistory` options.
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
	//   description: page size of results, maximum page size is 50
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
	// swagger:operation GET /catalog/{owner}/{repo} catalog catalogSearchRepo
	// ---
	// summary: Catalog search by repo
	// produces:
	// - application/json
	// parameters:
	// - name: q
	//   in: query
	//   description: keyword
	//   type: string
	// - name: topic
	//   in: query
	//   description: Limit search to repositories with keyword as topic
	//   type: boolean
	// - name: includeDesc
	//   in: query
	//   description: include search of keyword within repository description
	//   type: boolean
	// - name: stage
	//   in: query
	//   description: One ore more stages to return. Supported values are
	//                "prod" (production releases),
	//                "preprod" (pre-production releases),
	//                "draft" (draft releases), and
	//                "latest" (the default branch if it is a valid RC).
	//                Can have multiple stages given.
	//   type: string
	// - name: includeHistory
	//   in: query
	//   description: If true, all releases, not just the latest, are included
	//   type: bool
	// - name: searchAllMetadata
	//   in: query
	//   description: By default is true. If false, not all metadata values are searched, only subject and title
	//   type: bool
	// - name: lang
	//   in: query
	//   description: If the repo is a resource of the given language(s), the repo will be in the results. Multiple lang's are ORed.
	//   type: string
	// - name: subject
	//   in: query
	//   description: resource subject
	//   type: string
	// - name: book
	//   in: query
	//   description: book (project id) that exist in a resource. If the resource contains the
	//                the book, its repository will be included in the results
	//   type: string
	// - name: checking_level
	//   in: query
	//   description: Checking level of the resource can be 1, 2 or 3
	//   type: string
	// - name: sort
	//   in: query
	//   description: sort repos by attribute. Supported values are
	//                "alpha", "created", "updated", "size", and "id".
	//                Default is "alpha"
	//   type: string
	// - name: tag
	//   in: query
	//   description: A release tag that the catalog entry has to have. Useful with `subject` and `searchAllHistory` options.
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
	//   description: page size of results, maximum page size is 50
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
	// swagger:operation GET /catalog/{owner}/{repo}/{tag} catalog catalogGetCatalogEntry
	// ---
	// summary: Catalog entry
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogEntry"
	//   "422":
	//     "$ref": "#/responses/validationError"

	dm, err := models.GetDoor43MetadataByRepoIDAndTagName(ctx.Repo.Repository.ID, ctx.Repo.TagName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}
	ctx.JSON(http.StatusOK, dm.APIFormat())
}

// GetCatalogMetadata Get the metadata (RC 0.2.0 manifest) in JSON format for the given ownername, reponame and ref
func GetCatalogMetadata(ctx *context.APIContext) {
	// swagger:operation GET /catalog/{owner}/{repo}/{tag}/metadata catalog catalogGetMetadata
	// ---
	// summary: Catalog entry metadata
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogMetadata"
	//   "422":
	//     "$ref": "#/responses/validationError"

	dm, err := models.GetDoor43MetadataByRepoIDAndTagName(ctx.Repo.Repository.ID, ctx.Repo.TagName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}
	ctx.JSON(http.StatusOK, dm.Metadata)
}

func searchCatalog(ctx *context.APIContext) {
	var repoID int64
	var owner, repo string
	searchAllMetadata := true
	if ctx.Repo.Repository != nil {
		repoID = ctx.Repo.Repository.ID
	} else {
		if ctx.Params("ownername") != "" {
			owner = ctx.Params("ownername")
		} else {
			owner = ctx.Query("owner")
		}
		repo = ctx.Query("repo")
	}
	if ctx.Query("searchAllMetadata") != "" {
		searchAllMetadata = ctx.QueryBool("searchAllMetadata")
	}

	keywords := []string{}
	query := strings.Trim(ctx.Query("q"), " ")
	if query != "" {
		// Split keyword, keeping words in quotes
		r := csv.NewReader(strings.NewReader(query))
		r.Comma = ' ' // space
		keywords, err := r.Read()
		if err != nil {
			log.Error("Read: %v", err)
			keywords = append(keywords, query)
		}
	}

	opts := &models.SearchCatalogOptions{
		ListOptions:       utils.GetListOptions(ctx),
		Keywords:          keywords,
		Owner:             owner,
		Repo:              repo,
		RepoID:            repoID,
		Tags:              ctx.QueryStrings("tag"),
		Stages:            ctx.QueryStrings("stage"),
		IncludeHistory:    ctx.QueryBool("includeHistory"),
		Languages:         ctx.QueryStrings("lang"),
		Subject:           ctx.Query("subject"),
		Books:             ctx.QueryStrings("book"),
		CheckingLevel:     ctx.Query("checking_level"),
		SearchAllMetadata: searchAllMetadata,
	}

	var sortMode = ctx.Query("sort")
	if len(sortMode) > 0 {
		var sortOrder = ctx.Query("order")
		if len(sortOrder) == 0 {
			sortOrder = "asc"
		}
		if searchModeMap, ok := searchOrderByMap[sortOrder]; ok {
			if orderBy, ok := searchModeMap[sortMode]; ok {
				opts.OrderBy = orderBy
			} else {
				ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("Invalid sort mode: \"%s\"", sortMode))
				return
			}
		} else {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("Invalid sort order: \"%s\"", sortOrder))
			return
		}
	}

	dms, count, err := models.SearchCatalog(opts)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	results := make([]*api.Door43Metadata, len(dms))
	for i, dm := range dms {
		results[i] = dm.APIFormat()
	}

	ctx.SetLinkHeader(int(count), opts.PageSize)
	ctx.Header().Set("X-Total-Count", fmt.Sprintf("%d", count))
	ctx.JSON(http.StatusOK, api.CatalogSearchResults{
		OK:   true,
		Data: results,
	})
}
