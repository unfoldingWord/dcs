// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

// ToUserDCS convert models.User to api.User with DCS customized fields populated
// signed shall only be set if requester is logged in. authed shall only be set if user is site admin or user himself
func ToUserDCS(user, doer *user_model.User) *api.User {
	if user == nil {
		return nil
	}
	result := ToUser(user, doer)
	result.RepoLanguages = models.GetRepoLanguages(user)
	result.RepoSubjects = models.GetRepoSubjects(user)
	return result
}

// ToUsersDCS convert list of models.User to list of api.User with DCS Customizations
func ToUsersDCS(doer *user_model.User, users []*user_model.User) []*api.User {
	result := make([]*api.User, len(users))
	for i := range users {
		result[i] = ToUserDCS(users[i], doer)
	}
	return result
}
