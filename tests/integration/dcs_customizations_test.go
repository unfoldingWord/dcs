// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	door43metadata_model "gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertDCSDoor43MetadataFixture(t *testing.T) *repo_model.Door43Metadata {
	t.Helper()

	dm := &repo_model.Door43Metadata{
		RepoID:            1,
		ReleaseID:         1,
		Ref:               "v1.1",
		RefType:           "tag",
		CommitSHA:         "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Stage:             door43metadata_model.StageProd,
		MetadataType:      "rc",
		MetadataVersion:   "0.2",
		Subject:           "TSV Translation Notes",
		FlavorType:        "parascriptural",
		Flavor:            "x-TranslationNotes",
		Abbreviation:      "tn",
		Title:             "Test Translation Notes",
		Publisher:         "Test Publisher",
		Language:          "en",
		LanguageTitle:     "English",
		LanguageDirection: "ltr",
		LanguageIsGL:      true,
		ContentFormat:     "tsv9",
		CheckingLevel:     1,
		IsLatestForStage:  true,
		IsRepoMetadata:    true,
		Metadata: map[string]any{
			"dublin_core": map[string]any{
				"title": "Test Translation Notes",
			},
		},
		ReleaseDateUnix: timeutil.TimeStamp(946684800),
		CreatedUnix:     timeutil.TimeStamp(946684800),
	}
	require.NoError(t, repo_model.InsertDoor43Metadata(t.Context(), dm))
	return dm
}

func TestDCSWebRoutesSmoke(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	for _, path := range []string{
		"/about",
		"/tools",
		"/catalog",
		"/user2/repo1/metadata",
	} {
		MakeRequest(t, NewRequest(t, "GET", path), http.StatusOK)
	}
}

func TestDCSWebReleaseCatalogBadgeConsistency(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	insertDCSDoor43MetadataFixture(t)

	listResp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/releases"), http.StatusOK)
	assert.Contains(t, listResp.Body.String(), "Catalog (prod)")

	singleResp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/releases/tag/v1.1"), http.StatusOK)
	assert.Contains(t, singleResp.Body.String(), "Catalog (prod)")
	assert.NotContains(t, singleResp.Body.String(), "Invalid (prod)")
}

func TestDCSAPIRoutesSmoke(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	for _, path := range []string{
		"/api/v1/catalog",
		"/api/v1/catalog/search",
		"/api/v1/catalog/list/subjects",
		"/api/v1/catalog/list/owners",
		"/api/v1/catalog/list/languages",
		"/api/v1/catalog/list/metadata-types",
		"/api/v1/languages/langnames.json",
		"/api/v1/languages/langnames_keyed.json",
	} {
		MakeRequest(t, NewRequest(t, "GET", path), http.StatusOK)
	}

	for _, path := range []string{
		"/api/v1/catalog/entry/user2/repo1/v1.1",
		"/api/v1/catalog/metadata/user2/repo1/v1.1",
		"/api/v1/catalog/validation/user2/repo1/v1.1",
	} {
		MakeRequest(t, NewRequest(t, "GET", path), http.StatusNotFound)
	}
}

func TestDCSAPICatalogEntryEndpoints(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	dm := insertDCSDoor43MetadataFixture(t)

	entryResp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/catalog/entry/user2/repo1/v1.1"), http.StatusOK)
	var entry api.CatalogEntry
	DecodeJSON(t, entryResp, &entry)
	assert.Equal(t, dm.Ref, entry.Ref)
	assert.Equal(t, "prod", entry.Stage)

	metadataResp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/catalog/metadata/user2/repo1/v1.1"), http.StatusOK)
	var metadata map[string]any
	DecodeJSON(t, metadataResp, &metadata)
	assert.Contains(t, metadata, "dublin_core")

	validationResp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/catalog/validation/user2/repo1/v1.1"), http.StatusOK)
	var validation any
	DecodeJSON(t, validationResp, &validation)
	assert.Nil(t, validation)
}

func TestDCSAPIRepoHealthcheck(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	insertDCSDoor43MetadataFixture(t)

	resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/healthcheck"), http.StatusOK)
	var payload struct {
		OK bool `json:"ok"`
	}
	DecodeJSON(t, resp, &payload)
	assert.True(t, payload.OK)
}

func TestDCSWebRepoHealthcheckRefPage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	insertDCSDoor43MetadataFixture(t)

	// the ref-specific page renders the tag's own health check
	MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/healthcheck/v1.1"), http.StatusOK)
	// the overall page (repo's canonical entry) still renders
	MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/healthcheck"), http.StatusOK)
	// a ref with no catalog entry redirects to the metadata page
	resp := MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/healthcheck/no-such-ref"), http.StatusSeeOther)
	assert.Equal(t, "/user2/repo1/metadata", resp.Header().Get("Location"))
}
