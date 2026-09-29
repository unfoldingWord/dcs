// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

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

	aReq, err := sbarchiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, ctx.PathParam("*"))
	if err != nil {
		if errors.Is(err, sbarchiver_service.ErrUnknownArchiveFormat{}) {
			ctx.APIError(http.StatusBadRequest, err.Error())
		} else if errors.Is(err, sbarchiver_service.RepoRefNotFoundError{}) {
			ctx.APIError(http.StatusNotFound, err.Error())
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	sbarchiver_service.ServeRepoSBArchive(ctx.Base, ctx.Repo.Repository, aReq)
}
