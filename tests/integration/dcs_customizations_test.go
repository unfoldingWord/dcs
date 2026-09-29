// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"net/http"
	"regexp"
	"testing"

	auth_model "gitea.dev/models/auth"
	door43metadata_model "gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/setting"
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
	assert.Equal(t, setting.AppURL+"user2/repo1/archive/v1.1.zip", entry.ZipballURL)
	assert.Equal(t, setting.AppURL+"user2/repo1/sb/v1.1.zip", entry.SBZipballURL)
	assert.Equal(t, setting.AppURL+"user2/repo1/sb/v1.1.tar.gz", entry.SBTarballURL)

	metadataResp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/catalog/metadata/user2/repo1/v1.1"), http.StatusOK)
	var metadata map[string]any
	DecodeJSON(t, metadataResp, &metadata)
	assert.Contains(t, metadata, "dublin_core")

	validationResp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/catalog/validation/user2/repo1/v1.1"), http.StatusOK)
	var validation any
	DecodeJSON(t, validationResp, &validation)
	assert.Nil(t, validation)
}

// TestDCSAPIRepoSBArchive covers /api/v1/repos/{owner}/{repo}/sb/{ref}.{zip|tar.gz}, the API twin of the
// web /sb/ download. user2/repo1 is marked as already being in SB format so the archive is its tree as is.
func TestDCSAPIRepoSBArchive(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// No metadata at all: nothing to convert
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/master.zip"), http.StatusNotFound)

	require.NoError(t, repo_model.InsertDoor43Metadata(t.Context(), &repo_model.Door43Metadata{
		RepoID: 1, Ref: "master", RefType: "branch", CommitSHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Stage: door43metadata_model.StageLatest, MetadataType: "sb", MetadataVersion: "1.0.0",
		IsLatestForStage: true, IsRepoMetadata: true,
	}))

	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/master"), http.StatusBadRequest)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/master.bundle"), http.StatusBadRequest)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/no-such-ref.zip"), http.StatusNotFound)

	resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/master.zip"), http.StatusOK)
	assert.Equal(t, "application/zip", resp.Header().Get("Content-Type"))
	assert.Contains(t, resp.Header().Get("Content-Disposition"), "repo1-master-sb.zip")
	zr, err := zip.NewReader(bytes.NewReader(resp.Body.Bytes()), int64(resp.Body.Len()))
	require.NoError(t, err)
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	assert.Contains(t, names, "repo1/README.md")

	// The "immutable" link must point at this API route, pinned to the commit
	linkHeaderRe := regexp.MustCompile(`^<(https?://.*/api/v1/repos/user2/repo1/sb/[a-f0-9]+\.zip\?rev=[a-f0-9]+)>; rel="immutable"$`)
	m := linkHeaderRe.FindStringSubmatch(resp.Header().Get("Link"))
	require.Len(t, m, 2, "Link header %q", resp.Header().Get("Link"))
	pinned := MakeRequest(t, NewRequest(t, "GET", m[1]), http.StatusOK)
	assert.Equal(t, resp.Body.Bytes(), pinned.Body.Bytes())

	resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/master.tar.gz"), http.StatusOK)
	assert.Equal(t, "application/gzip", resp.Header().Get("Content-Type"))
	assert.Contains(t, resp.Header().Get("Content-Disposition"), "repo1-master-sb.tar.gz")
	_, err = gzip.NewReader(bytes.NewReader(resp.Body.Bytes()))
	require.NoError(t, err)

	MakeRequest(t, NewRequest(t, "HEAD", "/api/v1/repos/user2/repo1/sb/master.zip"), http.StatusOK)

	// Private repo: hidden anonymously, readable with a token (the web /sb/ route has no token auth)
	_ = repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), &repo_model.Repository{ID: 1, IsPrivate: true}, "is_private")
	MakeRequest(t, NewRequest(t, "HEAD", "/api/v1/repos/user2/repo1/sb/master.zip"), http.StatusNotFound)
	token := getTokenForLoggedInUser(t, loginUser(t, "user2"), auth_model.AccessTokenScopeReadRepository)
	MakeRequest(t, NewRequest(t, "HEAD", "/api/v1/repos/user2/repo1/sb/master.zip").AddTokenAuth(token), http.StatusOK)
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

func TestDCSAPICatalogSearchCaseInsensitiveFilters(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	dm := insertDCSDoor43MetadataFixture(t)

	entryRefs := func(query string) []string {
		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/catalog/search?"+query), http.StatusOK)
		var results api.CatalogSearchResults
		DecodeJSON(t, resp, &results)
		refs := make([]string, 0, len(results.Data))
		for _, entry := range results.Data {
			refs = append(refs, entry.Ref)
		}
		return refs
	}

	// the DB collation is case-sensitive, so subject/flavor/flavorType must match
	// regardless of the casing given
	for _, query := range []string{
		"subject=TSV%20Translation%20Notes",
		"subject=tsv%20translation%20notes",
		"subject=TSV%20TRANSLATION%20NOTES",
		"subject=translation%20notes&partialMatch=true",
		"subject=TRANSLATION%20NOTES&partialMatch=true",
		"flavorType=parascriptural",
		"flavorType=Parascriptural",
		"flavor=x-TranslationNotes",
		"flavor=x-translationnotes",
		"flavor=X-TRANSLATIONNOTES",
	} {
		assert.Contains(t, entryRefs(query), dm.Ref, "query: %s", query)
	}

	// a value differing by more than case still does not match
	assert.NotContains(t, entryRefs("subject=Aligned%20Bible"), dm.Ref)
	assert.NotContains(t, entryRefs("flavor=x-Bible"), dm.Ref)
}

func TestDCSAPIRepoSearchIsHealthy(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	dm := insertDCSDoor43MetadataFixture(t) // repo1's canonical entry, never checked

	repoNames := func(query string) []string {
		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/search?"+query), http.StatusOK)
		var payload struct {
			Data []struct {
				Name string `json:"name"`
			} `json:"data"`
		}
		DecodeJSON(t, resp, &payload)
		names := make([]string, 0, len(payload.Data))
		for _, r := range payload.Data {
			names = append(names, r.Name)
		}
		return names
	}

	// never-checked repos are not healthy
	assert.NotContains(t, repoNames("is_healthy=true"), "repo1")
	assert.Contains(t, repoNames("is_healthy=false"), "repo1")

	// a warning-severity canonical entry is healthy by default but not under the strict filter
	dm.HealthcheckSeverity = repo_model.SeverityLevelWarning
	require.NoError(t, repo_model.UpdateDoor43MetadataCols(t.Context(), dm, "healthcheck_severity"))
	assert.Contains(t, repoNames("is_healthy=true"), "repo1")
	assert.NotContains(t, repoNames("is_healthy_without_warnings=true"), "repo1")
}
