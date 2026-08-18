// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/dcs" // DCS Customizations
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	attachment_service "gitea.dev/services/attachment"
	"gitea.dev/services/context"
	"gitea.dev/services/context/upload"
	"gitea.dev/services/convert"
	door43metadata_service "gitea.dev/services/door43metadata" // DCS Customizations
	notify_service "gitea.dev/services/notify"
)

/*** DCS Customizations ***/

// notifyReleaseAttachmentChanged reacts to a release attachment being added,
// edited or deleted via the API. Upstream Gitea has no post-attachment
// processing, so the attachment endpoints never dispatched a release
// notification; only release create/edit did.
//
// For a files.json / links.json manifest it re-fires the release update
// notification so the door43 metadata notifier expands it into external-link
// assets and reprocesses the release's Door43Metadata. For any other asset it
// just recomputes the has_audio / has_video / has_pdf / has_stream / has_other
// content flags on the release's Door43Metadata.
func notifyReleaseAttachmentChanged(ctx *context.APIContext, releaseID int64, name string) {
	rel, err := repo_model.GetReleaseByID(ctx, releaseID)
	if err != nil {
		log.Error("notifyReleaseAttachmentChanged: GetReleaseByID [%d]: %v", releaseID, err)
		return
	}
	if rel.IsDraft {
		return
	}
	if dcs.IsJSONManifestAttachmentName(name) {
		rel.Repo = ctx.Repo.Repository
		if err := rel.LoadAttributes(ctx); err != nil {
			log.Error("notifyReleaseAttachmentChanged: LoadAttributes [%d]: %v", releaseID, err)
			return
		}
		notify_service.UpdateRelease(ctx, ctx.Doer, rel)
		return
	}
	if err := door43metadata_service.UpdateDoor43MetadataAttachmentFlags(ctx, rel.RepoID, rel.ID); err != nil {
		log.Error("notifyReleaseAttachmentChanged: UpdateDoor43MetadataAttachmentFlags [%d]: %v", releaseID, err)
	}
}

/*** END DCS Customizations ***/

func checkReleaseMatchRepo(ctx *context.APIContext, releaseID int64) bool {
	release, err := repo_model.GetReleaseByID(ctx, releaseID)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.APIErrorNotFound()
			return false
		}
		ctx.APIErrorInternal(err)
		return false
	}
	if release.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
		return false
	}
	if release.IsDraft && !canAccessReleaseDraft(ctx) {
		ctx.APIErrorNotFound()
		return false
	}
	return true
}

// GetReleaseAttachment gets a single attachment of the release
func GetReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoGetReleaseAttachment
	// ---
	// summary: Get a release attachment
	// produces:
	// - application/json
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Attachment"
	//   "404":
	//     "$ref": "#/responses/notFound"

	releaseID := ctx.PathParamInt64("id")
	if !checkReleaseMatchRepo(ctx, releaseID) {
		return
	}

	attachID := ctx.PathParamInt64("attachment_id")
	attach, err := repo_model.GetAttachmentByID(ctx, attachID)
	if err != nil {
		if repo_model.IsErrAttachmentNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	if attach.ReleaseID != releaseID {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", releaseID, attachID)
		ctx.APIErrorNotFound()
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests
	ctx.JSON(http.StatusOK, convert.ToAPIAttachment(ctx.Repo.Repository, attach))
}

// ListReleaseAttachments lists all attachments of the release
func ListReleaseAttachments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id}/assets repository repoListReleaseAttachments
	// ---
	// summary: List release's attachments
	// produces:
	// - application/json
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AttachmentList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	releaseID := ctx.PathParamInt64("id")
	release, err := repo_model.GetReleaseByID(ctx, releaseID)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	if release.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
		return
	}
	if release.IsDraft && !canAccessReleaseDraft(ctx) {
		ctx.APIErrorNotFound()
		return
	}
	if err := release.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIRelease(ctx, ctx.Repo.Repository, release).Attachments)
}

// CreateReleaseAttachment creates an attachment and saves the given file
func CreateReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/releases/{id}/assets repository repoCreateReleaseAttachment
	// ---
	// summary: Create a release attachment
	// produces:
	// - application/json
	// consumes:
	// - multipart/form-data
	// - application/octet-stream
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: name
	//   in: query
	//   description: name of the attachment
	//   type: string
	//   required: false
	// - name: attachment
	//   in: formData
	//   description: attachment to upload
	//   type: file
	//   required: false
	// responses:
	//   "201":
	//     "$ref": "#/responses/Attachment"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "413":
	//     "$ref": "#/responses/error"

	// Check if attachments are enabled
	if !setting.Attachment.Enabled {
		ctx.APIErrorNotFound("attachment is not enabled")
		return
	}

	// Check if release exists an load release
	releaseID := ctx.PathParamInt64("id")
	if !checkReleaseMatchRepo(ctx, releaseID) {
		return
	}

	// Get uploaded file from request
	var filename string
	var uploaderFile *attachment_service.UploaderFile
	if strings.HasPrefix(strings.ToLower(ctx.Req.Header.Get("Content-Type")), "multipart/form-data") {
		file, header, err := ctx.Req.FormFile("attachment")
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		defer file.Close()

		filename = header.Filename
		if name := ctx.FormString("name"); name != "" {
			filename = name
		}
		uploaderFile = attachment_service.NewLimitedUploaderKnownSize(file, header.Size)
	} else {
		filename = ctx.FormString("name")
		uploaderFile = attachment_service.NewLimitedUploaderMaxBytesReader(ctx.Req.Body, ctx.Resp)
	}

	if filename == "" {
		ctx.APIError(http.StatusBadRequest, "Could not determine name of attachment.")
		return
	}

	// Create a new attachment and save the file
	attach, err := attachment_service.UploadAttachmentForRelease(ctx, uploaderFile, &repo_model.Attachment{
		Name:       filename,
		UploaderID: ctx.Doer.ID,
		RepoID:     ctx.Repo.Repository.ID,
		ReleaseID:  releaseID,
	})
	if err != nil {
		if upload.IsErrFileTypeForbidden(err) {
			ctx.APIError(http.StatusBadRequest, err.Error())
			return
		}

		if errors.Is(err, util.ErrContentTooLarge) {
			ctx.APIError(http.StatusRequestEntityTooLarge, err.Error())
			return
		}

		ctx.APIErrorInternal(err)
		return
	}

	// DCS: expand a freshly uploaded files.json / links.json into link assets,
	// or update the Door43Metadata content flags for any other asset.
	notifyReleaseAttachmentChanged(ctx, releaseID, attach.Name)

	ctx.JSON(http.StatusCreated, convert.ToAPIAttachment(ctx.Repo.Repository, attach))
}

// EditReleaseAttachment updates the given attachment
func EditReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoEditReleaseAttachment
	// ---
	// summary: Edit a release attachment
	// produces:
	// - application/json
	// consumes:
	// - application/json
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditAttachmentOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Attachment"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm[*api.EditAttachmentOptions](ctx)

	// Check if release exists an load release
	releaseID := ctx.PathParamInt64("id")
	if !checkReleaseMatchRepo(ctx, releaseID) {
		return
	}

	attachID := ctx.PathParamInt64("attachment_id")
	attach, err := repo_model.GetAttachmentByID(ctx, attachID)
	if err != nil {
		if repo_model.IsErrAttachmentNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	if attach.ReleaseID != releaseID {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", releaseID, attachID)
		ctx.APIErrorNotFound()
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests
	if form.Name != "" {
		attach.Name = form.Name
	}

	if err := attachment_service.UpdateAttachment(ctx, setting.Repository.Release.AllowedTypes, attach); err != nil {
		if upload.IsErrFileTypeForbidden(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err.Error())
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	// DCS: expand a files.json / links.json into link assets when one is
	// created via rename/edit of an attachment, or update the Door43Metadata
	// content flags for any other asset.
	notifyReleaseAttachmentChanged(ctx, releaseID, attach.Name)

	ctx.JSON(http.StatusCreated, convert.ToAPIAttachment(ctx.Repo.Repository, attach))
}

// DeleteReleaseAttachment delete a given attachment
func DeleteReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoDeleteReleaseAttachment
	// ---
	// summary: Delete a release attachment
	// produces:
	// - application/json
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// Check if release exists an load release
	releaseID := ctx.PathParamInt64("id")
	if !checkReleaseMatchRepo(ctx, releaseID) {
		return
	}

	attachID := ctx.PathParamInt64("attachment_id")
	attach, err := repo_model.GetAttachmentByID(ctx, attachID)
	if err != nil {
		if repo_model.IsErrAttachmentNotExist(err) {
			ctx.APIErrorNotFound()
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	if attach.ReleaseID != releaseID {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", releaseID, attachID)
		ctx.APIErrorNotFound()
		return
	}

	if err := repo_model.DeleteAttachment(ctx, attach, true); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// DCS: update the Door43Metadata content flags now that an asset is gone.
	notifyReleaseAttachmentChanged(ctx, releaseID, attach.Name)

	ctx.Status(http.StatusNoContent)
}
