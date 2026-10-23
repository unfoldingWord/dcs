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

// DownloadSBFile serves one file of a repository's Scripture Burrito, as the API's repoGetSBFile.
func DownloadSBFile(ctx *context.Context) {
	err := sbarchiver_service.ServeSBFile(ctx.Base, ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.FormTrim("ref"), ctx.PathParam("*"))
	switch {
	case err == nil:
	case errors.Is(err, util.ErrInvalidArgument):
		ctx.HTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, util.ErrNotExist), errors.Is(err, sbarchiver_service.RepoRefNotFoundError{}), errors.Is(err, sbarchiver_service.ErrRepoNotConvertible{}):
		ctx.HTTPError(http.StatusNotFound, err.Error())
	default:
		ctx.ServerError("sbarchiver_service.ServeSBFile", err)
	}
}
