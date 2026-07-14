// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"net/url"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

// ToGitRef converts a git.Reference to a api.Reference
func ToGitRef(repo *repo_model.Repository, ref *git.Reference) *api.Reference {
	return &api.Reference{
		Ref: ref.Name,
		URL: repo.APIURL() + "/git/" + util.PathEscapeSegments(ref.Name),
		Object: &api.GitObject{
			SHA:  ref.Object.String(),
			Type: ref.Type,
			URL:  repo.APIURL() + "/git/" + url.PathEscape(ref.Type) + "s/" + url.PathEscape(ref.Object.String()),
		},
	}
}
