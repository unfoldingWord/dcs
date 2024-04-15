// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"

	user_model "code.gitea.io/gitea/models/user"
	metadata_service "code.gitea.io/gitea/services/door43metadata"
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

func registerLoadMetadataSchemasTask() {
	RegisterTaskFatal("load_schemas", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return metadata_service.LoadMetadataSchemas(ctx)
	})
}
