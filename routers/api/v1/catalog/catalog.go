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
	"code.gitea.io/gitea/modules/util"
)

var searchOrderByMap = map[string]map[string]door43metadata.CatalogOrderBy{
	"asc": {
		"title":    door43metadata.CatalogOrderByTitle,
		"subject":  door43metadata.CatalogOrderBySubject,
		"resource": door43metadata.CatalogOrderByResource,
		"reponame": door43metadata.CatalogOrderByRepoName,
		"released": door43metadata.CatalogOrderByOldest,
		"lang":     door43metadata.CatalogOrderByLangCode,
		"releases": door43metadata.CatalogOrderByReleases,
		"stars":    door43metadata.CatalogOrderByStars,
		"forks":    door43metadata.CatalogOrderByForks,
		"tag":      door43metadata.CatalogOrderByTag,
	},
	"desc": {
		"title":    door43metadata.CatalogOrderByTitleReverse,
		"subject":  door43metadata.CatalogOrderBySubjectReverse,
		"resouce":  door43metadata.CatalogOrderByResourceReverse,
		"reponame": door43metadata.CatalogOrderByRepoNameReverse,
		"released": door43metadata.CatalogOrderByNewest,
		"lang":     door43metadata.CatalogOrderByLangCodeReverse,
		"releases": door43metadata.CatalogOrderByReleasesReverse,
		"stars":    door43metadata.CatalogOrderByStarsReverse,
		"forks":    door43metadata.CatalogOrderByForksReverse,
		"tag":      door43metadata.CatalogOrderByTagReverse,
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
	//                "latest" - return the default branch (e.g. master) if it is a valid RC instead of the above'
	//   type: string
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: resource
	//   in: query
	//   description: resource identifier. Multiple resources are ORed.
	//   type: string
	// - name: format
	//   in: query
	//   description: content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (project ids). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: metadataType
	//   in: query
	//   description: return repos only with metadata of this type (e.g. rc, tc, ts, sb). <empty> or "all" for all. Default is rc
	//   type: string
	// - name: metadataVersion
	//   in: query
	//   description: return repos only with the version of metadata given. Does not apply if metadataType is "all"
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
	//   description: if true, the list of ingredients (files/projects) in the resource and their file paths will be listed for each entry. Default is true
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
	// - name: resource
	//   in: query
	//   description: resource identifier. Multiple resources are ORed.
	//   type: string
	// - name: format
	//   in: query
	//   description: content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (project ids). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: metadataType
	//   in: query
	//   description: return repos only with metadata of this type (e.g. rc, tc, ts, sb). . <empty> or "all" for all. Default is rc
	//   type: string
	// - name: metadataVersion
	//   in: query
	//   description: return repos only with the version of metadata given. Does not apply if metadataType is "all"
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
	//   description: if true, the list of ingredients (files/projects) in the resource and their file paths will be listed for each entry. Default is true
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
	// - name: resource
	//   in: query
	//   description: resource identifier. Multiple resources are ORed.
	//   type: string
	// - name: format
	//   in: query
	//   description: content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (project ids). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: metadataType
	//   in: query
	//   description: return repos only with metadata of this type (e.g. rc, tc, ts, sb). <empty> or "all" for all. Default is rc
	//   type: string
	// - name: metadataVersion
	//   in: query
	//   description: return repos only with the version of metadata given. Does not apply if metadataType is "all"
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
	//   description: if true, the list of ingredients (files/projects) in the resource and their file paths will be listed for each entry. Default is true
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

// ListCatalogSubjects list the subjects available in the catalog
func ListCatalogSubjects(ctx *context.APIContext) {
	// swagger:operation GET /catalog/subjects catalog catalogListSubjects
	// ---
	// summary: Catalog list subjects
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: query
	//   description: list only subjects in the given owner(s). To match multiple, give the parameter multiple times or give a list comma delimited.
	//   type: string
	// - name: lang
	//   in: query
	//   description: list only subjects in the given language(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: stage
	//   in: query
	//   description: 'list only subjects of the given stage or lower, with low to high being:
	//                "prod" - return only the production subjects (default);
	//                "preprod" - return pre-production and production subjects;
	//                "draft" - return only draft, pre-product and production subjects;
	//                "latest" - return all subjects in the catalog for all stages'
	//   type: string
	// - name: subject
	//   in: query
	//   description: list only the given subjects if they are in the catalog meeting the criteria given (e.g. way to test a given language has the given subject)
	//   type: string
	// - name: resource
	//   in: query
	//   description: list only subjects with the given resource identifier. Multiple resources are ORed.
	//   type: string
	// - name: format
	//   in: query
	//   description: list only subjects with the given content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: list only for subjects with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: list only subjects with the given book(s) (project ids). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: metadataType
	//   in: query
	//   description: list only subjects with the given metadata type (e.g. rc, tc, ts, sb). . <empty> or "all" for all. Default is rc
	//   type: string
	// - name: metadataVersion
	//   in: query
	//   description: list only subjects with the  given metadatay version. Does not apply if metadataType is "all" or empty
	//   type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, owner, subject and language search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// responses:
	//   "200":
	//     "$ref": "#/responses/StringSlice"
	//   "422":
	//     "$ref": "#/responses/validationError"

	listSingleDMField(ctx, "`door43_metadata`.subject")
}

// ListCatalogOwners list the subjects available in the catalog
func ListCatalogOwners(ctx *context.APIContext) {
	// swagger:operation GET /catalog/owners catalog catalogListOwners
	// ---
	// summary: Catalog list owners
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: query
	//   description: list only the given owners(s) if they have entries in the catalog meeting the criteria given (e.g. way to test an owner has a given language or subject)
	//   type: string
	// - name: lang
	//   in: query
	//   description: list only owners with entries in the given language(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: stage
	//   in: query
	//   description: 'list only owners of the given stage or lower, with low to high being:
	//                "prod" - return only the production subjects (default);
	//                "preprod" - return pre-production and production subjects;
	//                "draft" - return only draft, pre-product and production subjects;
	//                "latest" - return all subjects in the catalog for all stages'
	//   type: string
	// - name: subject
	//   in: query
	//   description: list only owners with the the given subject(s). Multiple resources are ORed.
	//   type: string
	// - name: resource
	//   in: query
	//   description: list only owners with the given resource identifier(s). Multiple resources are ORed.
	//   type: string
	// - name: format
	//   in: query
	//   description: list only owners with the given content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: list only for owners with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: list only owners with the given book(s) (project ids). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: metadataType
	//   in: query
	//   description: list only owners with the given metadata type (e.g. rc, tc, ts, sb). . <empty> or "all" for all. Default is rc
	//   type: string
	// - name: metadataVersion
	//   in: query
	//   description: list only owners with the  given metadatay version. Does not apply if metadataType is "all" or empty
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/StringSlice"
	//   "422":
	//     "$ref": "#/responses/validationError"

	listSingleDMField(ctx, "`user`.lower_name")
}

// ListCatalogLanguages list the languages available in the catalog
func ListCatalogLanguages(ctx *context.APIContext) {
	// swagger:operation GET /catalog/owners catalog catalogListLanguages
	// ---
	// summary: Catalog list owners
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: query
	//   description: list only lannguages with entries in the given owners(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: lang
	//   in: query
	//   description: list only the given languages(s) if they have entries in the catalog meeting the criteria given (e.g. way to test an language has a given owner and/or subject)
	//   type: string
	// - name: stage
	//   in: query
	//   description: 'list only languages of the given stage or lower, with low to high being:
	//                "prod" - return only the production subjects (default);
	//                "preprod" - return pre-production and production subjects;
	//                "draft" - return only draft, pre-product and production subjects;
	//                "latest" - return all subjects in the catalog for all stages'
	//   type: string
	// - name: subject
	//   in: query
	//   description: list only languages with the the given subject(s). Multiple resources are ORed.
	//   type: string
	// - name: resource
	//   in: query
	//   description: list only languages with the given resource identifier(s). Multiple resources are ORed.
	//   type: string
	// - name: format
	//   in: query
	//   description: list only languages with the given content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: list only for languages with the given checking level(s). Can be 1, 2 or 3
	//   type: string
	// - name: book
	//   in: query
	//   description: list only languages with the given book(s) (project ids). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: string
	// - name: metadataType
	//   in: query
	//   description: list only languages with the given metadata type (e.g. rc, tc, ts, sb). . <empty> or "all" for all. Default is rc
	//   type: string
	// - name: metadataVersion
	//   in: query
	//   description: list only languages with the  given metadatay version. Does not apply if metadataType is "all" or empty
	//   type: string
	// - name: direction
	//   in: query
	//   description: list only languages of the given language direction, "ltr" or "rtl".
	//   type: string
	// - name: isGL
	//   in: query
	//   description: list only languages of they are (true) or are not (false) a gatetway language.
	//   type: boolean
	// - name: partialMatch
	//   in: query
	//   description: if true, owner, language and subject search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// responses:
	//   "200":
	//     "$ref": "#/responses/StringSlice"
	//   "422":
	//     "$ref": "#/responses/validationError"

	listSingleDMField(ctx, "`door43_metadata`.language")
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
		dm.Repo = ctx.Repo.Repository
	}
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}
	if err := dm.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}
	accessMode, err := access_model.AccessLevel(ctx.ContextUser, dm.Repo)
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

	metadataTypes := QueryStrings(ctx, "metadataType")
	metadataVersions := QueryStrings(ctx, "metadataVersion")

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
		ListOptions:      listOptions,
		Keywords:         keywords,
		Owners:           owners,
		Repos:            repos,
		RepoID:           repoID,
		Tags:             QueryStrings(ctx, "tag"),
		Stage:            stage,
		Languages:        QueryStrings(ctx, "lang"),
		Subjects:         QueryStrings(ctx, "subject"),
		Resources:        QueryStrings(ctx, "resource"),
		ContentFormats:   QueryStrings(ctx, "format"),
		CheckingLevels:   QueryStrings(ctx, "checkingLevel"),
		Books:            QueryStrings(ctx, "book"),
		IncludeHistory:   ctx.FormBool("includeHistory"),
		ShowIngredients:  ctx.FormOptionalBool("showIngredients"),
		IncludeMetadata:  includeMetadata,
		MetadataTypes:    metadataTypes,
		MetadataVersions: metadataVersions,
		PartialMatch:     ctx.FormBool("partialMatch"),
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
		if ctx.Repo != nil && ctx.Repo.Repository != nil {
			dm.Repo = ctx.Repo.Repository
		} else {
			err := dm.LoadAttributes()
			if err != nil {
				ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("invalid Door43Metadata entry, id: %d", dm.ID))
				return
			}
		}
		accessMode, err := access_model.AccessLevel(ctx.ContextUser, dm.Repo)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, api.SearchError{
				OK:    false,
				Error: err.Error(),
			})
		}
		dmAPI := convert.ToCatalogEntry(dm, accessMode)
		if opts.ShowIngredients == util.OptionalBoolFalse {
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

func listSingleDMField(ctx *context.APIContext, field string) {
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

	metadataTypes := QueryStrings(ctx, "metadataType")
	metadataVersions := QueryStrings(ctx, "metadataVersion")
	if len(metadataTypes) == 1 && (metadataTypes[0] == "all" || metadataTypes[0] == "") {
		metadataTypes = []string{}
		metadataVersions = []string{}
	}

	listOptions := db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: ctx.FormInt("limit"),
	}
	if listOptions.Page < 1 {
		listOptions.Page = 1
	}

	opts := &door43metadata.SearchCatalogOptions{
		ListOptions:      listOptions,
		Owners:           QueryStrings(ctx, "owner"),
		Repos:            QueryStrings(ctx, "repos"),
		Tags:             QueryStrings(ctx, "tag"),
		Stage:            stage,
		Languages:        QueryStrings(ctx, "lang"),
		Subjects:         QueryStrings(ctx, "subject"),
		Resources:        QueryStrings(ctx, "resource"),
		ContentFormats:   QueryStrings(ctx, "format"),
		CheckingLevels:   QueryStrings(ctx, "checkingLevel"),
		Books:            QueryStrings(ctx, "book"),
		IncludeHistory:   ctx.FormBool("includeHistory"),
		ShowIngredients:  ctx.FormOptionalBool("showIngredients"),
		MetadataTypes:    metadataTypes,
		MetadataVersions: metadataVersions,
		PartialMatch:     ctx.FormBool("partialMatch"),
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

	results, err := models.SearchDoor43MetadataField(opts, field)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, api.SearchError{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	ctx.RespHeader().Set("X-Total-Count", fmt.Sprintf("%d", len(results)))
	ctx.JSON(http.StatusOK, results)
}
