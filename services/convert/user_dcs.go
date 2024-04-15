// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

func toUserDCS(ctx context.Context, user *user_model.User, apiUser *api.User) *api.User {
	if user != nil && apiUser != nil {
		apiUser.RepoLanguages = models.GetRepoLanguages(ctx, user)
		apiUser.RepoSubjects = models.GetRepoSubjects(ctx, user)
		apiUser.RepoMetadataTypes = models.GetRepoMetadataTypes(ctx, user)
	}
	return apiUser
}
