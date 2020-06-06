// Copyright 2020 The DCS Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	"encoding/json"
	"fmt"
)

const (
	tplDoor43Metadatas   base.TplName = "repo/metadata/list"
	tplDoor43MetadataNew base.TplName = "repo/metadata/new"
)

// Door43Metadatas render door43 metadata list page
func Door43Metadatas(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.metadata.metadatas")
	ctx.Data["PageIsDoor43MetadataList"] = true

	writeAccess := ctx.Repo.CanWrite(models.UnitTypeReleases)
	ctx.Data["CanCreateMetadata"] = writeAccess && !ctx.Repo.Repository.IsArchived

	opts := models.FindDoor43MetadatasOptions{
		ListOptions: models.ListOptions{
			Page:     ctx.QueryInt("page"),
			PageSize: convert.ToCorrectPageSize(ctx.QueryInt("limit")),
		},
	}

	if opts.ListOptions.Page <= 1 {
		opts.ListOptions.Page = 1
	}
	if opts.ListOptions.PageSize <= 0 {
		opts.ListOptions.Page = 10
	}

	metadatas, err := models.GetDoor43MetadatasByRepoID(ctx.Repo.Repository.ID, opts)
	if err != nil {
		ctx.ServerError("GetDoor43MetadatasByRepoID", err)
		return
	}

	for _, metadata := range metadatas {
		if err := metadata.LoadAttributes(); err != nil {
			ctx.ServerError("LoadAttributes", err)
			return
		}
	}

	count, err := models.GetDoor43MetadataCountByRepoID(ctx.Repo.Repository.ID, opts)
	if err != nil {
		ctx.ServerError("GetDoor43MetadataCountByRepoID", err)
		return
	}

	ctx.Data["Door43Metadatas"] = metadatas

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(200, tplDoor43Metadatas)
}

// SingleDoor43Metadata renders a single door43 metadata's page
func SingleDoor43Metadata(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.metadata.metadatas")
	ctx.Data["PageIsDoor43MetadataList"] = true

	writeAccess := ctx.Repo.CanWrite(models.UnitTypeReleases)
	ctx.Data["CanCreateMetadata"] = writeAccess && !ctx.Repo.Repository.IsArchived

	metadata, err := models.GetDoor43MetadataByRepoIDAndTagName(ctx.Repo.Repository.ID, ctx.Params("tag"))
	if err != nil {
		ctx.ServerError("GetDoor43MetadataByRepoIDAndTagName", err)
		return
	}

	ctx.Data["Door43Metadatas"] = []*models.Door43Metadata{metadata}
	ctx.HTML(200, tplDoor43Metadatas)
}

// LatestDoor43Metadata redirects to the latest door43 metadata
func LatestDoor43Metadata(ctx *context.Context) {
	metadata, err := models.GetLatestDoor43MetadataByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		if models.IsErrDoor43MetadataNotExist(err) {
			ctx.NotFound("LatestDoor43Metadata", err)
			return
		}
		ctx.ServerError("GetLatestDoor43MetadataByRepoID", err)
		return
	}

	if err := metadata.LoadAttributes(); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	ctx.Redirect(metadata.HTMLURL())
}

// NewDoor43Metadata render creating door43 metadata page
func NewDoor43Metadata(ctx *context.Context) {
	latestRelease, err := models.GetLatestReleaseByRepoID(ctx.Repo.Repository.ID)
	if err != nil && !models.IsErrReleaseNotExist(err) {
		ctx.ServerError("GetLatestReleaseByRepoID", err)
		return
	}
	releases, err := models.GetReleasesByRepoID(ctx.Repo.Repository.ID, models.FindReleasesOptions{})
	if err != nil {
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}

	ctx.Data["Title"] = ctx.Tr("repo.metadata.new_metadata")
	ctx.Data["Releases"] = releases
	ctx.Data["PageIsMetadataList"] = true
	ctx.Data["tag_name"] = latestRelease.TagName
	ctx.HTML(200, tplDoor43MetadataNew)
}

// NewDoor43MetadataPost response for creating a door43 metadata
func NewDoor43MetadataPost(ctx *context.Context, form auth.NewDoor43MetadataForm) {
	ctx.Data["Title"] = ctx.Tr("repo.metadata.new_metadata")
	ctx.Data["PageIsMetadataList"] = true

	if ctx.HasError() {
		ctx.HTML(200, tplDoor43MetadataNew)
		return
	}

	var releaseID int64
	if form.TagName != "" && form.TagName != "default" {
		release, err := models.GetRelease(ctx.Repo.Repository.ID, form.TagName)
		if err != nil {
			ctx.RenderWithErr(ctx.Tr("repo.metadata.tag_name_invalid"), tplDoor43MetadataNew, &form)
			return
		}
		releaseID = release.ID
	}

	var metadata map[string]interface{}
	err := json.Unmarshal([]byte(form.Metadata), &metadata)
	if err != nil {
		ctx.RenderWithErr(ctx.Tr("repo.metadata.metadata_not_proper_json", err), tplDoor43MetadataNew, &form)
		return
	}

	dm := &models.Door43Metadata{
		RepoID:    ctx.Repo.Repository.ID,
		ReleaseID: releaseID,
		Metadata:  metadata,
	}

	if isExist, err := models.IsDoor43MetadataExist(dm.RepoID, dm.ReleaseID); err != nil {
		ctx.ServerError("IsDoor43MetadataExist", err)
		return
	} else if isExist {
		ctx.RenderWithErr(ctx.Tr("repo.metadata.metadata_already_exist"), tplDoor43MetadataNew, &form)
		return
	}

	if err := models.InsertDoor43Metadata(dm); err != nil {
		ctx.ServerError("InsertDoor43Metadata", err)
		return
	}

	log.Trace("Door43Metadata created: %s/%s:%s", ctx.User.LowerName, ctx.Repo.Repository.Name, form.TagName)

	ctx.Redirect(ctx.Repo.RepoLink + "/metadatas")
}

// EditDoor43Metadata render door43 metadata edit page
func EditDoor43Metadata(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.metadata.edit_metadata")
	ctx.Data["PageIsMetadataList"] = true
	ctx.Data["PageIsEditMetadata"] = true
	renderAttachmentSettings(ctx)

	tagName := ctx.Params("*")
	dm, err := models.GetDoor43MetadataByRepoIDAndTagName(ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if models.IsErrDoor43MetadataNotExist(err) {
			ctx.NotFound("GetDoor43Metadata", err)
		} else {
			ctx.ServerError("GetDoor43Metadata", err)
		}
		return
	}

	metadata, err := json.MarshalIndent(dm.Metadata, "", "  ")
	if err != nil {
		ctx.ServerError(fmt.Sprintf("EditDoor43Metadata - Marshal [%d]: %v", dm.ID, err), err)
		return
	}

	ctx.Data["ID"] = dm.ID
	ctx.Data["tag_name"] = tagName
	ctx.Data["metadata"] = string(metadata)

	ctx.HTML(200, tplDoor43MetadataNew)
}

// EditDoor43MetadataPost response for edit door43 metadata
func EditDoor43MetadataPost(ctx *context.Context, form auth.EditDoor43MetadataForm) {
	ctx.Data["Title"] = ctx.Tr("repo.metadata.edit_metadata")
	ctx.Data["PageIsMetadataList"] = true
	ctx.Data["PageIsEditMetadata"] = true

	tagName := ctx.Params("*")
	dm, err := models.GetDoor43MetadataByRepoIDAndTagName(ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if models.IsErrDoor43MetadataNotExist(err) {
			ctx.NotFound("GetDoor43Metadata", err)
		} else {
			ctx.ServerError("GetDoor43Metadata", err)
		}
		return
	}
	ctx.Data["tag_name"] = tagName
	ctx.Data["metadata"] = dm.Metadata

	if ctx.HasError() {
		ctx.HTML(200, tplDoor43MetadataNew)
		return
	}

	if err = json.Unmarshal(form.Metadata, &dm.Metadata); err != nil {
		ctx.RenderWithErr(ctx.Tr("repo.metadata.metadata_not_proper_json"), tplDoor43MetadataNew, &form)
	}
	if err = models.UpdateDoor43MetadataCols(dm, "metadata"); err != nil {
		ctx.ServerError("UpdateDoor43Metadata", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/metadatas")
}

// DeleteDoor43Metadata delete a door43 metadata
func DeleteDoor43Metadata(ctx *context.Context) {
	if err := models.DeleteDoor43MetadataByID(ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteDoor43MetadataByID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.metadata.deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/metadatas",
	})
}
