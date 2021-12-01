// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	door43metadata_service "code.gitea.io/gitea/services/door43metadata"

	"github.com/urfave/cli"
)

// CmdDoor43MetadataGenerate represents the available door43-metadata-generate sub-command.
var CmdDoor43MetadataGenerate = cli.Command{
	Name:        "generate-door43-metadata",
	Usage:       "Generate Door43 Metadata",
	Description: "This is a command for generating door43 metadata, making sure all valid repos have metadata in the door43_metadata table.",
	Action:      runDoor43MetadataGenerate,
}

func runDoor43MetadataGenerate(ctx *cli.Context) error {
	stdCtx, cancel := installSignals()
	defer cancel()
	if err := initDB(stdCtx); err != nil {
		return err
	}

	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	setting.InitDBConfig()

	if err := db.InitEngineWithMigration(context.Background(), door43metadata_service.GenerateDoor43Metadata); err != nil {
		log.Fatal("Failed to initialize ORM engine: %v", err)
		return err
	}

	return nil
}
