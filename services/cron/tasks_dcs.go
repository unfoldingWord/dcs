// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"

	user_model "gitea.dev/models/user"
	convertrc2sb_service "gitea.dev/services/convertrc2sb"
	metadata_service "gitea.dev/services/door43metadata"
)

func registerUpdateDoor43MetadataTask() {
	RegisterTaskFatal("update_metadata", &BaseConfig{
		Enabled:    true,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return metadata_service.UpdateDoor43Metadata(ctx)
	})
}

func registerUpdateUserMetadataTask() {
	RegisterTaskFatal("update_user_metadata", &BaseConfig{
		Enabled:    true,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return metadata_service.UpdateUserMetadata(ctx)
	})
}

func registerLoadMetadataSchemasTask() {
	RegisterTaskFatal("load_schemas", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return metadata_service.LoadMetadataSchemas(ctx)
	})
}

func registerConvertRC2SBTask() {
	RegisterTaskFatal("convert_rc2sb", &BaseConfig{
		Enabled:    true,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return convertrc2sb_service.AllRepos(ctx)
	})
}
