// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

func toUserDCS(user *user_model.User, apiUser *api.User) *api.User {
	if user != nil && apiUser != nil {
		apiUser.RepoLanguages = models.GetRepoLanguages(user)
		apiUser.RepoSubjects = models.GetRepoSubjects(user)
		apiUser.RepoMetadataTypes = models.GetRepoMetadataTypes(user)
	}
	return apiUser
}
