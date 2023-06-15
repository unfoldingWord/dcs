// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
)

// NotifyNewDoor43Metadata places a place holder function
func (*NullNotifier) NotifyNewDoor43Metadata(doer *user_model.User, repo *repo.Repository, refType, refFullName string) {
}

// NotifyUpdateDoor43Metadata places a place holder function
func (*NullNotifier) NotifyUpdateDoor43Metadata(doer *user_model.User, repo *repo.Repository, refType, refFullName string) {
}

// NotifyDeleteDoor43Metadata places a place holder function
func (*NullNotifier) NotifyDeleteDoor43Metadata(doer *user_model.User, repo *repo.Repository, refType, refFullName string) {
}
