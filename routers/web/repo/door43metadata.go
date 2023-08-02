// Copyright 2023 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	door43metadata_service "code.gitea.io/gitea/services/door43metadata"
)

const (
	tplDoor43Metadata base.TplName = "repo/settings/dcs_metadata"
)

// Door43Metadta render door43 metadata page
func Door43Metadata(ctx *context.Context) {
	ctx.Data["Title"] = "Door43 Metadata"
	ctx.Data["PageIsSettingsDoor43Metadata"] = true
	if ctx.Repo.Repository.RepoDM != nil && ctx.Repo.Repository.RepoDM.Metadata != nil {
		if metadataBuf, err := json.MarshalIndent(ctx.Repo.Repository.RepoDM.Metadata, "", "    "); err != nil {
			log.Error("Door43Metadata: JSON parse error: %v", err)
			ctx.Data["Metadata"] = "There was an error reading the repo's metadata. Please try again."
		} else {
			ctx.Data["Metadata"] = string(metadataBuf)
		}
	}
	ctx.HTML(http.StatusOK, tplDoor43Metadata)
}

// UpdateDoor43Metadata updates the repo's metadata
func UpdateDoor43Metadata(ctx *context.Context) {
	if err := door43metadata_service.ProcessDoor43MetadataForRepo(ctx, ctx.Repo.Repository, ""); err != nil {
		ctx.Flash.Error("ProcessDoor43MetadataForRepo: " + err.Error())
	} else {
		if err := ctx.Repo.Repository.LoadLatestDMs(ctx); err != nil {
			ctx.Flash.Warning("Error loading metadata. Please try again.")
		} else if ctx.Repo.Repository.RepoDM.Metadata != nil {
			ctx.Flash.Success("Successfully scanned this repo's metadata.")
		} else {
			ctx.Flash.Warning("No metadata found!")
		}
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/metadata")
}
