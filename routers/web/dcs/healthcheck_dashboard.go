// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"net/http"
	"strings"

	"gitea.dev/models"
	"gitea.dev/models/db"
	"gitea.dev/models/door43metadata"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"

	"xorm.io/builder"
)

const (
	tplHCDash      templates.TplName = "catalog/hc_dash"
	hcDashPageSize int               = 100
)

// HealthcheckDashboard renders the healthcheck dashboard page.
func HealthcheckDashboard(ctx *context.Context) {
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	// Defaults: tc-ready filter ON, failed-only filter OFF, all metadata types.
	filtersApplied := ctx.FormString("filters_applied") == "1"
	tcReadyOnly := true
	failedOnly := false
	if filtersApplied {
		tcReadyOnly = ctx.FormString("tc_ready") == "1"
		failedOnly = ctx.FormString("failed_only") == "1"
	}
	metadataType := ctx.FormString("metadata_type")
	switch metadataType {
	case "rc", "ts", "tc", "sb":
	default:
		metadataType = ""
	}

	opts := &door43metadata.SearchCatalogOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: hcDashPageSize,
		},
		Stage:          door43metadata.StagePreProd,
		IncludeHistory: false,
		Healthchecks:   []string{"success,error,warning,info"},
		OrderBy:        []door43metadata.CatalogOrderBy{door43metadata.CatalogOrderByNewest},
	}
	if metadataType != "" {
		opts.MetadataTypes = []string{metadataType}
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
		ownerOrRepoCond := builder.NewCond().
			Or(door43metadata.LikeCond("`repository`.lower_name", keyword)).
			Or(door43metadata.LikeCond("`user`.lower_name", keyword)).
			Or(door43metadata.LikeCond("LOWER(`door43_metadata`.subject)", keyword))
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
	ctx.Data["MetadataType"] = metadataType
	ctx.Data["Door43Metadatas"] = dms
	ctx.Data["Total"] = count

	ctx.Data["Page"] = context.NewPagerBuilder(ctx).TotalCount(count).PerPageLimit(hcDashPageSize).CurPage(page).Build()

	ctx.HTML(http.StatusOK, tplHCDash)
}
