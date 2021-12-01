// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
)

/*** DCS Customizations ***/

// NotifyNewDoor43Metadata places a place holder function
func (*NullNotifier) NotifyNewDoor43Metadata(doer *user_model.User, repo *models.Repository, refType, refFullName string) {
}

// NotifyUpdateDoor43Metadata places a place holder function
func (*NullNotifier) NotifyUpdateDoor43Metadata(doer *user_model.User, repo *models.Repository, refType, refFullName string) {
}

// NotifyDeleteDoor43Metadata places a place holder function
func (*NullNotifier) NotifyDeleteDoor43Metadata(doer *user_model.User, repo *models.Repository, refType, refFullName string) {
}
