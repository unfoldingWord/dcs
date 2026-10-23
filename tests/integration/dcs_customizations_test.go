// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	door43metadata_model "gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
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

func TestDCSAPIFileSaveProcessesMetadata(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		token := getTokenForLoggedInUser(t, loginUser(t, "user2"), auth_model.AccessTokenScopeWriteRepository)
		opts := getCreateFileOptions()
		opts.NewBranchName = "dcs-metadata-save"
		opts.ContentBase64 = base64.StdEncoding.EncodeToString([]byte("dublin_core:\n  identifier: ult\n  language: {identifier: en, title: English, direction: ltr}\n  subject: Bible\n  title: Test Bible\nprojects: []\n"))
		MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/contents/manifest.yaml", &opts).AddTokenAuth(token), http.StatusCreated)

		// test queues are immediate, so the push has been processed by the time the save returns
		dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(t.Context(), 1, "dcs-metadata-save")
		require.NoError(t, err)
		assert.Equal(t, "rc", dm.MetadataType)
	})
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

func sbZipEntry(t *testing.T, archive []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	rc, err := zr.Open(name)
	require.NoError(t, err)
	defer rc.Close()
	content, err := io.ReadAll(rc)
	require.NoError(t, err)
	return content
}

// TestDCSAPIRepoSBFile covers /api/v1/repos/{owner}/{repo}/sb/raw/{filepath} and its web twin for a repo already
// in SB format, and checks that a file has the bytes of the same path in the archive.
func TestDCSAPIRepoSBFile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// No metadata at all: nothing to convert
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/README.md"), http.StatusNotFound)

	require.NoError(t, repo_model.InsertDoor43Metadata(t.Context(), &repo_model.Door43Metadata{
		RepoID: 1, Ref: "master", RefType: "branch", CommitSHA: "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Stage: door43metadata_model.StageLatest, MetadataType: "sb", MetadataVersion: "1.0.0",
		IsLatestForStage: true, IsRepoMetadata: true,
	}))

	archive := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/master.zip"), http.StatusOK)
	assert.NotEmpty(t, archive.Header().Get("ETag"))
	assert.Equal(t, "max-age=0, public, must-revalidate", archive.Header().Get("Cache-Control"))
	readme := sbZipEntry(t, archive.Body.Bytes(), "repo1/README.md")

	resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/README.md"), http.StatusOK)
	assert.Equal(t, readme, resp.Body.Bytes())
	assert.Contains(t, resp.Header().Get("Content-Type"), "text/plain")
	assert.Equal(t, "max-age=0, public, must-revalidate", resp.Header().Get("Cache-Control"))
	assert.NotEmpty(t, resp.Header().Get("Last-Modified"))
	etag := resp.Header().Get("ETag")
	require.NotEmpty(t, etag)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/README.md").SetHeader("If-None-Match", etag), http.StatusNotModified)

	// A full commit ID can't change; a tag can be deleted and made again
	for ref, cacheControl := range map[string]string{
		"65f1bf27bc3bf70f64657658635e66094edbcb4d": "public, max-age=31536000, immutable",
		"v1.1": "max-age=0, public, must-revalidate",
	} {
		resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/README.md?ref="+ref), http.StatusOK)
		assert.Equal(t, readme, resp.Body.Bytes(), ref)
		assert.Equal(t, cacheControl, resp.Header().Get("Cache-Control"), ref)
	}

	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/ingredients/%2E%2E/README.md"), http.StatusBadRequest)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/no-such-file.md"), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/README.md?ref=no-such-ref"), http.StatusNotFound)

	// An archive of a ref named "raw" is still an archive request
	resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw.zip"), http.StatusNotFound)
	assert.Contains(t, resp.Body.String(), "unrecognized repository reference: raw")

	resp = MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/sb/raw/README.md?ref=v1.1"), http.StatusOK)
	assert.Equal(t, readme, resp.Body.Bytes())
	MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/sb/raw/no-such-file.md"), http.StatusNotFound)
}

// TestDCSAPIRepoSBFileConverted reads files of an rc repo, which come out of its converted SB archive.
func TestDCSAPIRepoSBFileConverted(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		token := getTokenForLoggedInUser(t, loginUser(t, "user2"), auth_model.AccessTokenScopeWriteRepository)
		manifest := "dublin_core:\n  conformsto: rc0.2\n  identifier: tn\n  language: {identifier: en, title: English, direction: ltr}\n" +
			"  subject: TSV Translation Notes\n  title: Test Notes\nprojects:\n  - {identifier: tit, path: ./tn_TIT.tsv, title: Titus}\n"
		notes := "Reference\tID\tTags\tSupportReference\tQuote\tOccurrence\tNote\n1:1\tabcd\t\t\t\t0\tA note\n"
		MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/contents", &api.ChangeFilesOptions{
			FileOptions: api.FileOptions{BranchName: "master", NewBranchName: "sb-tn", Message: "Add notes"},
			Files: []*api.ChangeFileOperation{
				{Operation: "create", Path: "manifest.yaml", ContentBase64: base64.StdEncoding.EncodeToString([]byte(manifest))},
				{Operation: "create", Path: "tn_TIT.tsv", ContentBase64: base64.StdEncoding.EncodeToString([]byte(notes))},
			},
		}).AddTokenAuth(token), http.StatusCreated)
		dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(t.Context(), 1, "sb-tn")
		require.NoError(t, err)
		require.Equal(t, "rc", dm.MetadataType)

		archive := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/sb-tn.zip"), http.StatusOK)
		for filePath, contentType := range map[string]string{"metadata.json": "application/json", "ingredients/TIT.tsv": "text/tab-separated-values"} {
			resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/"+filePath+"?ref=sb-tn"), http.StatusOK)
			assert.Equal(t, sbZipEntry(t, archive.Body.Bytes(), "repo1/"+filePath), resp.Body.Bytes(), filePath)
			assert.Equal(t, contentType, resp.Header().Get("Content-Type"), filePath)
		}
		assert.Equal(t, notes, string(sbZipEntry(t, archive.Body.Bytes(), "repo1/ingredients/TIT.tsv")))

		// The entries carry the commit time, not the time of the conversion
		commitTime, err := http.ParseTime(archive.Header().Get("Last-Modified"))
		require.NoError(t, err)
		zr, err := zip.NewReader(bytes.NewReader(archive.Body.Bytes()), int64(archive.Body.Len()))
		require.NoError(t, err)
		for _, f := range zr.File {
			assert.True(t, commitTime.Equal(f.Modified), "%s: %v", f.Name, f.Modified)
		}

		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/ingredients/TIT.tsv?ref="+dm.CommitSHA), http.StatusOK)
		assert.Equal(t, notes, resp.Body.String())
		assert.Equal(t, "public, max-age=31536000, immutable", resp.Header().Get("Cache-Control"))
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/ingredients/GEN.tsv?ref=sb-tn"), http.StatusNotFound)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/ingredients?ref=sb-tn"), http.StatusNotFound)

		// A new commit on the branch is read from a new archive, not the one kept for the old commit
		etag := resp.Header().Get("ETag")
		var contents api.ContentsResponse
		DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/contents/tn_TIT.tsv?ref=sb-tn"), http.StatusOK), &contents)
		updated := notes + "1:2\tefgh\t\t\t\t0\tAnother note\n"
		MakeRequest(t, NewRequestWithJSON(t, "PUT", "/api/v1/repos/user2/repo1/contents/tn_TIT.tsv", &api.UpdateFileOptions{
			FileOptions:   api.FileOptions{BranchName: "sb-tn", Message: "Add a note"},
			SHA:           contents.SHA,
			ContentBase64: base64.StdEncoding.EncodeToString([]byte(updated)),
		}).AddTokenAuth(token), http.StatusOK)
		resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/ingredients/TIT.tsv?ref=sb-tn"), http.StatusOK)
		assert.Equal(t, updated, resp.Body.String())
		assert.NotEqual(t, etag, resp.Header().Get("ETag"))

		// A tag made again on another commit serves that commit, and was never cached as immutable
		for _, target := range []string{dm.CommitSHA, "sb-tn"} {
			MakeRequest(t, NewRequest(t, "DELETE", "/api/v1/repos/user2/repo1/tags/moved-tag").AddTokenAuth(token), NoExpectedStatus)
			MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/tags", &api.CreateTagOption{TagName: "moved-tag", Target: target}).AddTokenAuth(token), http.StatusCreated)
			resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/ingredients/TIT.tsv?ref=moved-tag"), http.StatusOK)
			assert.Equal(t, util.Iif(target == "sb-tn", updated, notes), resp.Body.String(), target)
			assert.Equal(t, "max-age=0, public, must-revalidate", resp.Header().Get("Cache-Control"))
		}

		// A repo already in SB format is read from its archive too, so .gitattributes apply as in /sb/{ref}.zip
		resp = MakeRequest(t, NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/contents", &api.ChangeFilesOptions{
			FileOptions: api.FileOptions{BranchName: "master", NewBranchName: "sb-attr", Message: "Add attributes"},
			Files: []*api.ChangeFileOperation{
				{Operation: "create", Path: ".gitattributes", ContentBase64: base64.StdEncoding.EncodeToString([]byte("excluded.md export-ignore\nversion.txt export-subst\n"))},
				{Operation: "create", Path: "excluded.md", ContentBase64: base64.StdEncoding.EncodeToString([]byte("not archived\n"))},
				{Operation: "create", Path: "version.txt", ContentBase64: base64.StdEncoding.EncodeToString([]byte("$Format:%H$\n"))},
			},
		}).AddTokenAuth(token), http.StatusCreated)
		var changed api.FilesResponse
		DecodeJSON(t, resp, &changed)
		_, err = db.GetEngine(t.Context()).Delete(&repo_model.Door43Metadata{RepoID: 1, Ref: "sb-attr"})
		require.NoError(t, err)
		require.NoError(t, repo_model.InsertDoor43Metadata(t.Context(), &repo_model.Door43Metadata{
			RepoID: 1, Ref: "sb-attr", RefType: "branch", CommitSHA: changed.Commit.SHA,
			Stage: door43metadata_model.StageOther, MetadataType: "sb", MetadataVersion: "1.0.0",
		}))
		resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/version.txt?ref=sb-attr"), http.StatusOK)
		assert.Equal(t, changed.Commit.SHA+"\n", resp.Body.String())
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/excluded.md?ref=sb-attr"), http.StatusNotFound)

		// Without CACHE_SB_ARCHIVES each request converts anew and nothing is kept
		defer test.MockVariableValue(&setting.DCS.CacheSBArchives, false)()
		kept := unittest.GetCount(t, &repo_model.RepoArchiver{RepoID: 1})
		resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/raw/ingredients/TIT.tsv?ref=sb-tn"), http.StatusOK)
		assert.Equal(t, updated, resp.Body.String())
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/sb/sb-tn.zip"), http.StatusOK)
		assert.Equal(t, kept, unittest.GetCount(t, &repo_model.RepoArchiver{RepoID: 1}))
	})
}

func TestDCSAPIRepoHealthcheck(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	insertDCSDoor43MetadataFixture(t)

	resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/healthcheck"), http.StatusOK)
	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Checks []struct {
				IssueCode     string `json:"issue_code"`
				SeverityLevel string `json:"severity_level"`
				PositiveTitle string `json:"positive_title"`
				NegativeTitle string `json:"negative_title"`
			} `json:"checks"`
		} `json:"data"`
	}
	DecodeJSON(t, resp, &payload)
	assert.True(t, payload.OK)
	require.NotEmpty(t, payload.Data.Checks)
	for _, check := range payload.Data.Checks {
		assert.NotEmpty(t, check.SeverityLevel, check.IssueCode)
		assert.NotEmpty(t, check.PositiveTitle, check.IssueCode)
		assert.NotEmpty(t, check.NegativeTitle, check.IssueCode)
	}
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

// TestDCSAPICatalogBookPackage checks the ref each partner repo contributes to a book
// package and the request forms of /catalog/bp. repo1 (TSV Translation Notes, with the
// v1.1 release of the fixture helper and a master branch) is the requested repo; user2's
// public utf8 repo (33) is a Translation Words partner with a master branch and a v1
// release that is closer in time to repo1's master than utf8's own master is.
func TestDCSAPICatalogBookPackage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	tn := insertDCSDoor43MetadataFixture(t)

	insert := func(repoID int64, ref, refType string, stage door43metadata_model.Stage, latest bool, released timeutil.TimeStamp, subject string) {
		require.NoError(t, repo_model.InsertDoor43Metadata(t.Context(), &repo_model.Door43Metadata{
			RepoID: repoID, Ref: ref, RefType: refType, CommitSHA: tn.CommitSHA,
			Stage: stage, IsLatestForStage: latest, ReleaseDateUnix: released,
			MetadataType: "rc", Language: "en", Subject: subject,
		}))
	}
	insert(1, "master", "branch", door43metadata_model.StageLatest, true, tn.ReleaseDateUnix+100, tn.Subject)
	insert(33, "master", "branch", door43metadata_model.StageLatest, true, tn.ReleaseDateUnix+5000, "Translation Words")
	insert(33, "v1", "tag", door43metadata_model.StageProd, true, tn.ReleaseDateUnix+10, "Translation Words")

	refsFor := func(pathAndQuery string) map[string]string {
		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/catalog/bp/user2/"+pathAndQuery), http.StatusOK)
		var results api.CatalogSearchResults
		DecodeJSON(t, resp, &results)
		refs := make(map[string]string, len(results.Data))
		for _, entry := range results.Data {
			refs[entry.Repo.Name] = entry.Ref
		}
		return refs
	}

	// A branch pairs with the partner's same-named branch, not its release that is
	// closer in time; the ref may be given in the path or as ?ref=.
	assert.Equal(t, map[string]string{"repo1": "master", "utf8": "master"}, refsFor("repo1/master"))
	assert.Equal(t, map[string]string{"repo1": "master", "utf8": "master"}, refsFor("repo1?ref=master"))

	// A release pairs with the partner's release closest in time.
	assert.Equal(t, map[string]string{"repo1": "v1.1", "utf8": "v1"}, refsFor("repo1/v1.1"))

	// The documented singular filter names and their plural aliases both apply.
	assert.Equal(t, map[string]string{"utf8": "master"}, refsFor("repo1/master?subject=Translation%20Words"))
	assert.Equal(t, map[string]string{"utf8": "master"}, refsFor("repo1/master?subjects=Translation%20Words"))

	// The removed tag= option is ignored rather than failing the request.
	assert.Equal(t, map[string]string{"repo1": "master", "utf8": "master"}, refsFor("repo1/master?tag=v1"))
}
