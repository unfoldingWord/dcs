// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/dcs"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/convert"
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
	// summary: Search the catalog
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
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: is_gl
	//   in: query
	//   description: list only those that are (true) or are not (false) a gatetway language
	//   type: boolean
	// - name: stage
	//   in: query
	//   description: 'specifies which release stage to be return of these stages:
	//                "prod" - return only the production releases (default);
	//                "preprod" - return the pre-production release if it exists instead of the production release;
	//                "latest" - return the default branch (e.g. master) if it is a valid repo'
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [prod,preprod,latest]
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [Aligned Bible,Aramaic Grammar,Bible,Greek Grammar,Greek Lexicon,Greek New Testament,Hebrew Grammar,Hebrew Old Testament,Hebrew-Aramaic Lexicon,OBS Study Notes,OBS Study Questions,OBS Translation Notes,OBS Translation Questions,Open Bible Stories,Study Notes,Study Questions,Training Library,Translation Academy,Translation Notes,Translation Questions,Translation Words,TSV Study Notes,TSV Study Questions,TSV Translation Notes,TSV Translation Questions,TSV Translation Words Links,TSV OBS Study Notes,TSV OBS Study Questions,TSV OBS Translation Notes,TSV OBS Translation Questions,TSV OBS Translation Words Links]
	// - name: resource
	//   in: query
	//   description: resource identifier. Multiple resources are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [glt,gst,obs,obs-sn,obs-sq,obs-tn,obs-tq,obs-twl,sn,sq,ta,tn,tq,tw,twl,ugnt,uhb,ult,ust]
	// - name: format
	//   in: query
	//   description: content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: ["1","2","3"]
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (ingredient identifiers). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: metadataType
	//   in: query
	//   description: return repos only with metadata of this type
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [rc,sb,tc,ts]
	// - name: metadataVersion
	//   in: query
	//   description: return repos only with the version of metadata given. Does not apply if metadataType is not given
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, subject, owner and repo search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// - name: includeHistory
	//   in: query
	//   description: if true, all releases, not just the latest, are included. Default is false
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
	// summary: Search the catalog by owner
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
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: is_gl
	//   in: query
	//   description: list only those that are (true) or are not (false) a gatetway language
	//   type: boolean
	// - name: stage
	//   in: query
	//   description: 'specifies which release stage to be return of these stages:
	//                "prod" - return only the production releases (default);
	//                "preprod" - return the pre-production release if it exists instead of the production release;
	//                "latest" -return the default branch (e.g. master) if it is a valid RC instead of the above'
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [prod,preprod,latest]
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [Aligned Bible,Aramaic Grammar,Bible,Greek Grammar,Greek Lexicon,Greek New Testament,Hebrew Grammar,Hebrew Old Testament,Hebrew-Aramaic Lexicon,OBS Study Notes,OBS Study Questions,OBS Translation Notes,OBS Translation Questions,Open Bible Stories,Study Notes,Study Questions,Training Library,Translation Academy,Translation Notes,Translation Questions,Translation Words,TSV Study Notes,TSV Study Questions,TSV Translation Notes,TSV Translation Questions,TSV Translation Words Links,TSV OBS Study Notes,TSV OBS Study Questions,TSV OBS Translation Notes,TSV OBS Translation Questions,TSV OBS Translation Words Links]
	// - name: resource
	//   in: query
	//   description: resource identifier. Multiple resources are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [glt,gst,obs,obs-sn,obs-sq,obs-tn,obs-tq,obs-twl,sn,sq,ta,tn,tq,tw,twl,ugnt,uhb,ult,ust]
	// - name: format
	//   in: query
	//   description: content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: ["1","2","3"]
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (ingredient identifiers). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: metadataType
	//   in: query
	//   description: return repos only with metadata of this type
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [rc,sb,tc,ts]
	// - name: metadataVersion
	//   in: query
	//   description: return repos only with the version of metadata given. Does not apply if metadataType is not given
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, subject, owner and repo search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// - name: includeHistory
	//   in: query
	//   description: if true, all releases, not just the latest, are included. Default is false
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
	// summary: Search the catalog by owner and repo
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
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: is_gl
	//   in: query
	//   description: list only those that are (true) or are not (false) a gatetway language
	//   type: boolean
	// - name: stage
	//   in: query
	//   description: 'specifies which release stage to be return of these stages:
	//                "prod" - return only the production releases (default);
	//                "preprod" - return the pre-production release if it exists instead of the production release;
	//                "latest" - return the default branch (e.g. master) if it is a valid repo'
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [prod,preprod,latest]
	// - name: subject
	//   in: query
	//   description: search only for entries with the given subject(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [Aligned Bible,Aramaic Grammar,Bible,Greek Grammar,Greek Lexicon,Greek New Testament,Hebrew Grammar,Hebrew Old Testament,Hebrew-Aramaic Lexicon,OBS Study Notes,OBS Study Questions,OBS Translation Notes,OBS Translation Questions,Open Bible Stories,Study Notes,Study Questions,Training Library,Translation Academy,Translation Notes,Translation Questions,Translation Words,TSV Study Notes,TSV Study Questions,TSV Translation Notes,TSV Translation Questions,TSV Translation Words Links,TSV OBS Study Notes,TSV OBS Study Questions,TSV OBS Translation Notes,TSV OBS Translation Questions,TSV OBS Translation Words Links]
	// - name: resource
	//   in: query
	//   description: resource identifier. Multiple resources are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [glt,gst,obs,obs-sn,obs-sq,obs-tn,obs-tq,obs-twl,sn,sq,ta,tn,tq,tw,twl,ugnt,uhb,ult,ust]
	// - name: format
	//   in: query
	//   description: content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: checkingLevel
	//   in: query
	//   description: search only for entries with the given checking level(s)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: ["1","2","3"]
	// - name: book
	//   in: query
	//   description: search only for entries with the given book(s) (ingredient identifiers). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: metadataType
	//   in: query
	//   description: return repos only with metadata of this type
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [rc,sb,tc,ts]
	// - name: metadataVersion
	//   in: query
	//   description: return repos only with the version of metadata given. Does not apply if metadataType is not given
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, subject, owner and repo search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// - name: includeHistory
	//   in: query
	//   description: if true, all releases, not just the latest, are included. Default is false
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
	// swagger:operation GET /catalog/list/subjects catalog catalogListSubjects
	// ---
	// summary: List of subjects in the catalog based on the given criteria
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
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: is_gl
	//   in: query
	//   description: list only those that are (true) or are not (false) a gatetway language
	//   type: boolean
	// - name: stage
	//   in: query
	//   description: 'list only those of the given stage or lower, with low to high being:
	//                "prod" - return only the production subjects (default);
	//                "preprod" - return pre-production and production subjects;
	//                "latest" - return all subjects in the catalog for all stages'
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [prod,preprod,latest]
	// - name: subject
	//   in: query
	//   description: list only the those if they are in the catalog meeting the criteria given (e.g. way to test a given language has the given subject)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [Aligned Bible,Aramaic Grammar,Bible,Greek Grammar,Greek Lexicon,Greek New Testament,Hebrew Grammar,Hebrew Old Testament,Hebrew-Aramaic Lexicon,OBS Study Notes,OBS Study Questions,OBS Translation Notes,OBS Translation Questions,Open Bible Stories,Study Notes,Study Questions,Training Library,Translation Academy,Translation Notes,Translation Questions,Translation Words,TSV Study Notes,TSV Study Questions,TSV Translation Notes,TSV Translation Questions,TSV Translation Words Links,TSV OBS Study Notes,TSV OBS Study Questions,TSV OBS Translation Notes,TSV OBS Translation Questions,TSV OBS Translation Words Links]
	// - name: resource
	//   in: query
	//   description: list only those with the given resource identifier. Multiple resources are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [glt,gst,obs,obs-sn,obs-sq,obs-tn,obs-tq,obs-twl,sn,sq,ta,tn,tq,tw,twl,ugnt,uhb,ult,ust]
	// - name: format
	//   in: query
	//   description: list only those with the given content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: list only those with the given checking level(s)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: ["1","2","3"]
	// - name: book
	//   in: query
	//   description: list only those with the given book(s) (ingredient identifiers). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: metadataType
	//   in: query
	//   description: list only those with the given metadata type
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [rc,sb,tc,ts]
	// - name: metadataVersion
	//   in: query
	//   description: list only those with the given metadatay version. Does not apply if metadataType is not given
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, owner, subject and language search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// responses:
	//   "200":
	//     description: "SearchResults of a successful catalog owner search"
	//     schema:
	//       type: object
	//       properties:
	//         ok:
	//           type: boolean
	//         data:
	//           type: array
	//           items:
	//             "$ref": "#/responses/StringSlice"

	list, err := getSingleDMFieldList(ctx, "`door43_metadata`.subject")
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
	}
	ctx.RespHeader().Set("X-Total-Count", fmt.Sprintf("%d", len(list)))
	ctx.JSON(http.StatusOK, map[string]any{
		"ok":   true,
		"data": list,
	})
}

// ListCatalogMetadataTypes list the metadata types available in the catalog
func ListCatalogMetadataTypes(ctx *context.APIContext) {
	// swagger:operation GET /catalog/list/metadata-types catalog catalogListMetadataTypes
	// ---
	// summary: List of metadata types in the catalog based on the given criteria
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
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: is_gl
	//   in: query
	//   description: list only those that are (true) or are not (false) a gatetway language
	//   type: boolean
	// - name: stage
	//   in: query
	//   description: 'list only those of the given stage or lower, with low to high being:
	//                "prod" - return only the production subjects (default);
	//                "preprod" - return pre-production and production subjects;
	//                "latest" - return all subjects in the catalog for all stages'
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [prod,preprod,latest]
	// - name: subject
	//   in: query
	//   description: list only the those if they are in the catalog meeting the criteria given (e.g. way to test a given language has the given subject)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [Aligned Bible,Aramaic Grammar,Bible,Greek Grammar,Greek Lexicon,Greek New Testament,Hebrew Grammar,Hebrew Old Testament,Hebrew-Aramaic Lexicon,OBS Study Notes,OBS Study Questions,OBS Translation Notes,OBS Translation Questions,Open Bible Stories,Study Notes,Study Questions,Training Library,Translation Academy,Translation Notes,Translation Questions,Translation Words,TSV Study Notes,TSV Study Questions,TSV Translation Notes,TSV Translation Questions,TSV Translation Words Links,TSV OBS Study Notes,TSV OBS Study Questions,TSV OBS Translation Notes,TSV OBS Translation Questions,TSV OBS Translation Words Links]
	// - name: resource
	//   in: query
	//   description: list only those with the given resource identifier. Multiple resources are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [glt,gst,obs,obs-sn,obs-sq,obs-tn,obs-tq,obs-twl,sn,sq,ta,tn,tq,tw,twl,ugnt,uhb,ult,ust]
	// - name: format
	//   in: query
	//   description: list only those with the given content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: list only those with the given checking level(s)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: ["1","2","3"]
	// - name: book
	//   in: query
	//   description: list only those with the given book(s) (ingredient identifiers). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: metadataType
	//   in: query
	//   description: list only those with the given metadata type
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [rc,sb,tc,ts]
	// - name: metadataVersion
	//   in: query
	//   description: list only those with the given metadatay version. Does not apply if metadataType is not given
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: partialMatch
	//   in: query
	//   description: if true, owner, subject and language search fields will use partial match (LIKE) when querying the catalog. Default is false
	//   type: boolean
	// responses:
	//   "200":
	//     description: "SearchResults of a successful catalog owner search"
	//     schema:
	//       type: object
	//       properties:
	//         ok:
	//           type: boolean
	//         data:
	//           type: array
	//           items:
	//             "$ref": "#/responses/StringSlice"

	list, err := getSingleDMFieldList(ctx, "`door43_metadata`.metadata_type")
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
	}
	ctx.RespHeader().Set("X-Total-Count", fmt.Sprintf("%d", len(list)))
	ctx.JSON(http.StatusOK, map[string]any{
		"ok":   true,
		"data": list,
	})
}

// ListCatalogOwners list the owners available in the catalog
func ListCatalogOwners(ctx *context.APIContext) {
	// swagger:operation GET /catalog/list/owners catalog catalogListOwners
	// ---
	// summary: List owners in the catalog based on the given criteria
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: query
	//   description: list only the given owners(s) if they have entries in the catalog meeting the criteria given (e.g. way to test an owner has a given language or subject)
	//   type: string
	// - name: lang
	//   in: query
	//   description: list only those with entries in the given language(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: is_gl
	//   in: query
	//   description: list only those that are (true) or are not (false) a gatetway language
	//   type: boolean
	// - name: stage
	//   in: query
	//   description: 'list only those of the given stage or lower, with low to high being:
	//                "prod" - return only the production subjects (default);
	//                "preprod" - return pre-production and production subjects;
	//                "latest" - return all subjects in the catalog for all stages'
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [prod,preprod,latest]
	// - name: subject
	//   in: query
	//   description: list only the those if they are in the catalog meeting the criteria given (e.g. way to test a given language has the given subject)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [Aligned Bible,Aramaic Grammar,Bible,Greek Grammar,Greek Lexicon,Greek New Testament,Hebrew Grammar,Hebrew Old Testament,Hebrew-Aramaic Lexicon,OBS Study Notes,OBS Study Questions,OBS Translation Notes,OBS Translation Questions,Open Bible Stories,Study Notes,Study Questions,Training Library,Translation Academy,Translation Notes,Translation Questions,Translation Words,TSV Study Notes,TSV Study Questions,TSV Translation Notes,TSV Translation Questions,TSV Translation Words Links,TSV OBS Study Notes,TSV OBS Study Questions,TSV OBS Translation Notes,TSV OBS Translation Questions,TSV OBS Translation Words Links]
	// - name: resource
	//   in: query
	//   description: list only those with the given resource identifier. Multiple resources are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [glt,gst,obs,obs-sn,obs-sq,obs-tn,obs-tq,obs-twl,sn,sq,ta,tn,tq,tw,twl,ugnt,uhb,ult,ust]
	// - name: format
	//   in: query
	//   description: list only those with the given content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: list only those with the given checking level(s)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: ["1","2","3"]
	// - name: book
	//   in: query
	//   description: list only those with the given book(s) (ingredient identifiers). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: metadataType
	//   in: query
	//   description: list only those with the given metadata type
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [rc,sb,tc,ts]
	// - name: metadataVersion
	//   in: query
	//   description: list only those with the given metadatay version. Does not apply if metadataType is not given
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// responses:
	//   "200":
	//     description: "SearchResults of a successful catalog owner search"
	//     schema:
	//       type: object
	//       properties:
	//         ok:
	//           type: boolean
	//         data:
	//           type: array
	//           items:
	//             "$ref": "#/responses/UserList"

	stageStr := ctx.FormString("stage")
	stage := door43metadata.StageProd
	if stageStr != "" {
		var ok bool
		stage, ok = door43metadata.StageMap[stageStr]
		if !ok {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("invalid stage [%s]", stageStr))
			return
		}
	}

	users, maxResults, err := user_model.SearchUsers(&user_model.SearchUserOptions{
		Actor:   ctx.Doer,
		Keyword: ctx.FormTrim("q"),
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		// DCS Customizations
		RepoLanguages:     ctx.FormStrings("lang"),
		RepoSubjects:      ctx.FormStrings("subject"),
		RepoMetadataTypes: ctx.FormStrings("metadata_type"),
		RepoLanguageIsGL:  ctx.FormOptionalBool("is_gl"),
		RepoCatalogStage:  stage,
		// END DCS Customizations
	})

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	ctx.SetLinkHeader(int(maxResults), len(users))
	ctx.SetTotalCountHeader(maxResults)

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok":   true,
		"data": convert.ToUsers(ctx, ctx.Doer, users),
	})
}

// ListCatalogLanguages list the languages available in the catalog
func ListCatalogLanguages(ctx *context.APIContext) {
	// swagger:operation GET /catalog/list/languages catalog catalogListLanguages
	// ---
	// summary: List languages in the catalog based on the given criteria
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: query
	//   description: list only those with the given owners(s). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive) unlesss partialMatch=true
	//   type: string
	// - name: lang
	//   in: query
	//   description: list only the given languages(s) if they have entries in the catalog meeting the criteria given (e.g. way to test an language has a given owner and/or subject)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: is_gl
	//   in: query
	//   description: list only those that are (true) or are not (false) a gatetway language
	//   type: boolean
	// - name: stage
	//   in: query
	//   description: 'list only those of the given stage or lower, with low to high being:
	//                "prod" - return only the production subjects (default);
	//                "preprod" - return pre-production and production subjects;
	//                "latest" - return all subjects in the catalog for all stages'
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [prod,preprod,latest]
	// - name: subject
	//   in: query
	//   description: list only the those if they are in the catalog meeting the criteria given (e.g. way to test a given language has the given subject)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [Aligned Bible,Aramaic Grammar,Bible,Greek Grammar,Greek Lexicon,Greek New Testament,Hebrew Grammar,Hebrew Old Testament,Hebrew-Aramaic Lexicon,OBS Study Notes,OBS Study Questions,OBS Translation Notes,OBS Translation Questions,Open Bible Stories,Study Notes,Study Questions,Training Library,Translation Academy,Translation Notes,Translation Questions,Translation Words,TSV Study Notes,TSV Study Questions,TSV Translation Notes,TSV Translation Questions,TSV Translation Words Links,TSV OBS Study Notes,TSV OBS Study Questions,TSV OBS Translation Notes,TSV OBS Translation Questions,TSV OBS Translation Words Links]
	// - name: resource
	//   in: query
	//   description: list only those with the given resource identifier. Multiple resources are ORed.
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [glt,gst,obs,obs-sn,obs-sq,obs-tn,obs-tq,obs-twl,sn,sq,ta,tn,tq,tw,twl,ugnt,uhb,ult,ust]
	// - name: format
	//   in: query
	//   description: list only those with the given content format (usfm, text, markdown, etc.). Multiple formats are ORed.
	//   type: string
	// - name: checkingLevel
	//   in: query
	//   description: list only those with the given checking level(s)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: ["1","2","3"]
	// - name: book
	//   in: query
	//   description: list only those with the given book(s) (ingredient identifiers). To match multiple, give the parameter multiple times or give a list comma delimited. Will perform an exact match (case insensitive)
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: metadataType
	//   in: query
	//   description: list only those with the given metadata type
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [rc,sb,tc,ts]
	// - name: metadataVersion
	//   in: query
	//   description: list only those with the given metadatay version. Does not apply if metadataType is not given
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// responses:
	//   "200":
	//     description: "SearchResults of a successful catalog owner search"
	//     schema:
	//       type: object
	//       properties:
	//         ok:
	//           type: boolean
	//         data:
	//           type: array
	//           items:
	//             "$ref": "#/responses/Language"

	list, err := getSingleDMFieldList(ctx, "`door43_metadata`.language")
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
	}
	var languages []map[string]interface{}
	langnames := dcs.GetLangnamesJSONKeyed()
	for _, lang := range list {
		if val, ok := langnames[lang]; ok {
			languages = append(languages, val)
		}
	}
	ctx.RespHeader().Set("X-Total-Count", fmt.Sprintf("%d", len(list)))
	ctx.JSON(http.StatusOK, map[string]any{
		"ok":   true,
		"data": languages,
	})
}

// GetCatalogEntry Get the catalog entry from the given ownername, reponame and ref
func GetCatalogEntry(ctx *context.APIContext) {
	// swagger:operation GET /catalog/entry/{owner}/{repo}/{ref} catalog catalogGetEntry
	// ---
	// summary: Get a catalog entry
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
	// - name: ref
	//   in: path
	//   description: release tag or default branch
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogEntry"
	//   "404":
	//     "$ref": "#/responses/notFound"

	ref := ctx.Params("ref")
	var dm *repo.Door43Metadata
	var err error
	dm, err = repo.GetDoor43MetadataByRepoIDAndRef(ctx, ctx.Repo.Repository.ID, ref)
	if err != nil {
		if !repo.IsErrDoor43MetadataNotExist(err) {
			ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndRef", err)
		} else {
			ctx.NotFound()
		}
		return
	}
	if err := dm.LoadAttributes(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	accessMode, err := access_model.AccessLevel(ctx, ctx.ContextUser, dm.Repo)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToCatalogEntry(ctx, dm, accessMode))
}

// GetCatalogMetadata Get the metadata (RC 0.2 manifest) in JSON format for the given ownername, reponame and ref
func GetCatalogMetadata(ctx *context.APIContext) {
	// swagger:operation GET /catalog/entry/{owner}/{repo}/{ref}/metadata catalog catalogGetMetadata
	// ---
	// summary: Get the metdata metadata (metadata.json or manifest.yaml in JSON format) of a catalog entry
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
	// - name: ref
	//   in: path
	//   description: release tag or default branch
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CatalogMetadata"
	//   "404":
	//     "$ref": "#/responses/notFound"

	ref := ctx.Params("ref")
	dm, err := repo.GetDoor43MetadataByRepoIDAndRef(ctx, ctx.Repo.Repository.ID, ref)
	if err != nil {
		if !repo.IsErrDoor43MetadataNotExist(err) {
			ctx.Error(http.StatusInternalServerError, "GetDoor43MetadataByRepoIDAndRef", err)
		} else {
			ctx.NotFound()
		}
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

	stageStr := ctx.FormString("stage")
	stage := door43metadata.StageProd
	if stageStr != "" {
		var ok bool
		stage, ok = door43metadata.StageMap[stageStr]
		if !ok {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("invalid stage [%s]", stageStr))
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
		LanguageIsGL:     ctx.FormOptionalBool("is_gl"),
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
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("invalid sort order [%s]", sortOrder))
			return
		}
	} else {
		opts.OrderBy = []door43metadata.CatalogOrderBy{door43metadata.CatalogOrderByLangCode, door43metadata.CatalogOrderBySubject, door43metadata.CatalogOrderByReleaseDateReverse}
	}

	dms, count, err := models.SearchCatalog(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchCatalog", err)
		return
	}

	results := make([]*api.CatalogEntry, len(dms))
	var lastUpdated time.Time
	for i, dm := range dms {
		if ctx.Repo != nil && ctx.Repo.Repository != nil {
			dm.Repo = ctx.Repo.Repository
		} else {
			err := dm.LoadAttributes(ctx)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
				return
			}
		}
		accessMode, err := access_model.AccessLevel(ctx, ctx.ContextUser, dm.Repo)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermissions", err)
			return
		}
		dmAPI := convert.ToCatalogEntry(ctx, dm, accessMode)
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

func getSingleDMFieldList(ctx *context.APIContext, field string) ([]string, error) {
	stageStr := ctx.FormString("stage")
	stage := door43metadata.StageProd
	if stageStr != "" {
		var ok bool
		stage, ok = door43metadata.StageMap[stageStr]
		if !ok {
			err := fmt.Errorf("invalid stage [%s]", stageStr)
			return nil, err
		}
	}

	metadataTypes := QueryStrings(ctx, "metadataType")
	metadataVersions := QueryStrings(ctx, "metadataVersion")

	listOptions := db.ListOptions{
		ListAll: true,
	}

	opts := &door43metadata.SearchCatalogOptions{
		ListOptions:      listOptions,
		Owners:           QueryStrings(ctx, "owner"),
		Repos:            QueryStrings(ctx, "repos"),
		Tags:             QueryStrings(ctx, "tag"),
		Stage:            stage,
		Languages:        QueryStrings(ctx, "lang"),
		LanguageIsGL:     ctx.FormOptionalBool("is_gl"),
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
					err := fmt.Errorf("invalid sort mode [%s]", sortMode)
					ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
						"ok":    false,
						"error": err.Error(),
					})
					return nil, err
				}
			}
		} else {
			err := fmt.Errorf("invalid sort order [%s]", sortOrder)
			return nil, err
		}
	} else {
		opts.OrderBy = []door43metadata.CatalogOrderBy{door43metadata.CatalogOrderByLangCode, door43metadata.CatalogOrderBySubject, door43metadata.CatalogOrderByReleaseDateReverse}
	}

	results, err := models.SearchDoor43MetadataField(ctx, opts, field)
	if err != nil {
		return nil, err
	}

	return results, nil
}
