// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/door43revisions"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

// CmdDoor43RevisionsGenerate represents the available door43-revisions-generate sub-command.
var CmdDoor43RevisionsGenerate = cli.Command{
	Name:        "generate-door43-revisions",
	Usage:       "Generate Door43 Revisions",
	Description: "This is a command for generating door43 revisions, making sure all valid repos have revisions in the door43_revision table.",
	Action:      runDoor43RevisionsGenerate,
}

func runDoor43RevisionsGenerate(ctx *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	setting.InitDBConfig()

	if err := models.NewEngine(context.Background(), door43revisions.GenerateDoor43Revisions); err != nil {
		log.Fatal("Failed to initialize ORM engine: %v", err)
		return err
	}

	return nil
}
