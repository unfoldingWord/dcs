// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/door43metadata"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"

	"xorm.io/builder"
)

const (
	tplHCDash      templates.TplName = "catalog/hc_dash"
	hcDashPageSize                   = 100
)

// HealthcheckDashboard renders the healthcheck dashboard page.
func HealthcheckDashboard(ctx *context.Context) {
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	// Defaults: tc-ready filter ON, failed-only filter OFF.
	filtersApplied := ctx.FormString("filters_applied") == "1"
	tcReadyOnly := true
	failedOnly := false
	if filtersApplied {
		tcReadyOnly = ctx.FormString("tc_ready") == "1"
		failedOnly = ctx.FormString("failed_only") == "1"
	}

	opts := &door43metadata.SearchCatalogOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: hcDashPageSize,
		},
		Stage:          door43metadata.StagePreProd,
		IncludeHistory: false,
		MetadataTypes:  []string{"rc"},
		Healthchecks:   []string{"success,error,warning,info"},
		OrderBy:        []door43metadata.CatalogOrderBy{door43metadata.CatalogOrderByNewest},
	}
	if tcReadyOnly {
		opts.Topics = []string{"tc-ready"}
	}
	if failedOnly {
		opts.Healthchecks = []string{"error,warning,info"}
	}

	cond := door43metadata.SearchCatalogCondition(opts)
	keyword := ctx.FormTrim("q")
	if keyword != "" {
		keyword = strings.ToLower(keyword)
		likePattern := "%" + keyword + "%"
		ownerOrRepoCond := builder.NewCond().
			Or(builder.Like{"`repository`.lower_name", likePattern}).
			Or(builder.Like{"`user`.lower_name", likePattern}).
			Or(builder.Expr("LOWER(`door43_metadata`.subject) LIKE ?", likePattern))
		cond = cond.And(ownerOrRepoCond)
	}

	dms, count, err := models.SearchCatalogByCondition(ctx, opts, cond)
	if err != nil {
		ctx.ServerError("SearchCatalogByCondition", err)
		return
	}

	ctx.Data["Title"] = "Healthcheck Dashboard"
	ctx.Data["Keyword"] = keyword
	ctx.Data["TCReadyOnly"] = tcReadyOnly
	ctx.Data["FailedOnly"] = failedOnly
	ctx.Data["Door43Metadatas"] = dms
	ctx.Data["Total"] = count

	pager := context.NewPagination(int(count), hcDashPageSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplHCDash)
}
