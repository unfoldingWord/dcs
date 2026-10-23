// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	sbarchiver_service "gitea.dev/services/repository/sbarchiver"
)

// GetSBArchive get a Scripture Burrito archive of a repository
func GetSBArchive(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/sb/{archive} repository repoGetSBArchive
	// ---
	// summary: Get a Scripture Burrito archive of a repository
	// description: |
	//   Returns the repository at the given git reference as a Scripture Burrito (SB) archive.
	//   Resource Container (rc), translationStudio (ts) and translationCore (tc) repositories are
	//   converted to SB on the fly; repositories already in SB format are archived as they are.
	//   Any other repository returns 404. The same archive can be downloaded without an API token
	//   from the repository's HTML URL, `/{owner}/{repo}/sb/{archive}`. The plain (unconverted) git
	//   archive is available from `/repos/{owner}/{repo}/archive/{archive}`.
	// produces:
	// - application/zip
	// - application/gzip
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: archive
	//   in: path
	//   description: the git reference (branch, tag or commit) with the archive format appended, either .zip or .tar.gz (e.g. master.zip, v1.tar.gz)
	//   type: string
	//   required: true
	// responses:
	//   200:
	//     description: the Scripture Burrito archive; Content-Disposition names it {repo}-{ref}-sb.{zip|tar.gz}
	//     schema:
	//       type: file
	//   "400":
	//     description: the archive format is not .zip or .tar.gz
	//   "404":
	//     description: the ref does not exist, or the repository is not an rc, ts, tc or sb repository

	aReq, err := sbarchiver_service.NewRequest(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.PathParam("*"))
	if err != nil {
		if errors.Is(err, sbarchiver_service.ErrUnknownArchiveFormat{}) {
			ctx.APIError(http.StatusBadRequest, err.Error())
		} else if errors.Is(err, sbarchiver_service.RepoRefNotFoundError{}) || errors.Is(err, sbarchiver_service.ErrRepoNotConvertible{}) {
			ctx.APIError(http.StatusNotFound, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	sbarchiver_service.ServeRepoSBArchive(ctx.Base, ctx.Repo.Repository, aReq)
}

// GetSBFile gets one file of a repository's Scripture Burrito
func GetSBFile(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/sb/raw/{filepath} repository repoGetSBFile
	// ---
	// summary: Get a file of a repository's Scripture Burrito
	// description: |
	//   Returns one file of the repository's Scripture Burrito (SB) at the given git reference: its
	//   `metadata.json` or an ingredient. The bytes are those of the same path inside the archive from
	//   `/repos/{owner}/{repo}/sb/{ref}.zip`, below its root folder, so a client can read single files or the
	//   whole archive interchangeably. Repositories already in SB format are read from git at that commit.
	//   The Content-Type is the ingredient's `mimeType` from `metadata.json`, and `application/json` for
	//   `metadata.json` itself. A tag or a full commit ID gets a long-lived, immutable Cache-Control; a branch
	//   is revalidated with the ETag. The same file can be read without an API token from the repository's
	//   HTML URL, `/{owner}/{repo}/sb/raw/{filepath}?ref={ref}`.
	// produces:
	// - application/octet-stream
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: filepath
	//   in: path
	//   description: path of the file inside the burrito, e.g. metadata.json or ingredients/TIT.tsv
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: the git reference (branch, tag or commit). Defaults to the repository's default branch
	//   type: string
	//   required: false
	// responses:
	//   200:
	//     description: the file
	//     schema:
	//       type: file
	//   "304":
	//     description: the copy named by If-None-Match is current
	//   "400":
	//     description: the path is empty, absolute or has a `..` segment
	//   "404":
	//     description: the ref does not exist, the repository is not an rc, ts, tc or sb repository, or the file is not in the burrito

	err := sbarchiver_service.ServeSBFile(ctx.Base, ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.FormTrim("ref"), ctx.PathParam("*"))
	switch {
	case err == nil:
	case errors.Is(err, util.ErrInvalidArgument):
		ctx.APIError(http.StatusBadRequest, err.Error())
	case errors.Is(err, util.ErrNotExist), errors.Is(err, sbarchiver_service.RepoRefNotFoundError{}), errors.Is(err, sbarchiver_service.ErrRepoNotConvertible{}):
		ctx.APIError(http.StatusNotFound, err.Error())
	default:
		ctx.APIErrorInternal(err)
	}
}
