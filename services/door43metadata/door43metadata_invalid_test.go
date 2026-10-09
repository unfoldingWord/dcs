// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.dev/models/door43metadata"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Minimal documents that pass the bundled schemas; the tests fail if a schema change
// makes them invalid, which is the point: they double as a check of the local schemas.
const validSBMetadata = `{
  "format": "scripture burrito",
  "meta": {
    "version": "1.0.0",
    "category": "source",
    "generator": {"softwareName": "DCS", "softwareVersion": "1.0", "userName": "test"},
    "defaultLocale": "en",
    "dateCreated": "2024-01-01T00:00:00.000Z",
    "normalization": "NFC",
    "comments": []
  },
  "idAuthorities": {"dcs": {"id": "https://git.door43.org", "name": {"en": "Door43 Content Service"}}},
  "identification": {
    "primary": {"dcs": {"user2/repo1": {"revision": "1", "timestamp": "2024-01-01T00:00:00.000Z"}}},
    "name": {"en": "Test Bible"},
    "abbreviation": {"en": "TB"}
  },
  "confidential": false,
  "type": {"flavorType": {"name": "scripture", "flavor": {"name": "textTranslation", "usfmVersion": "3.0", "translationType": "firstTranslation", "audience": "common", "projectType": "standard"}, "currentScope": {"GEN": []}}},
  "languages": [{"tag": "en", "name": {"en": "English"}, "scriptDirection": "ltr"}],
  "copyright": {"shortStatements": [{"statement": "CC BY-SA 4.0", "lang": "en"}]},
  "localizedNames": {"book-gen": {"short": {"en": "Genesis"}, "abbr": {"en": "Gen"}, "long": {"en": "Genesis"}}},
  "ingredients": {"ingredients/GEN.usfm": {"checksum": {"md5": "d41d8cd98f00b204e9800998ecf8427e"}, "mimeType": "text/x-usfm", "size": 0, "scope": {"GEN": []}}}
}`

const validRCManifest = `dublin_core:
  conformsto: rc0.2
  contributor: []
  creator: Test
  description: Test
  format: text/usfm
  identifier: ult
  issued: '2024-01-01'
  language:
    direction: ltr
    identifier: en
    title: English
  modified: '2024-01-01'
  publisher: Test Publisher
  relation: []
  rights: CC BY-SA 4.0
  source: []
  subject: Bible
  title: Test Bible
  type: bundle
  version: '1'
checking:
  checking_entity: []
  checking_level: '3'
projects:
  - categories: []
    identifier: gen
    path: ./01-GEN.usfm
    sort: 1
    title: Genesis
    versification: ufw
`

const validTcManifest = `{
  "tc_version": 8,
  "target_language": {"id": "en", "name": "English", "direction": "ltr"},
  "project": {"id": "gen", "name": "Genesis"},
  "resource": {"id": "ult", "name": "unfoldingWord Literal Text"}
}`

func TestPopulateSBDoor43Metadata(t *testing.T) {
	ctx := t.Context()
	repo := testRepo(1)

	t.Run("unparseable JSON is recorded as an invalid sb entry", func(t *testing.T) {
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateSBDoor43Metadata(ctx, nil, dm, repo, nil, []byte(`{"format": "scripture burrito",`)))
		assert.Equal(t, "sb", dm.MetadataType)
		assert.Equal(t, "1.0.0", dm.MetadataVersion)
		assert.Nil(t, dm.Metadata)
		require.NotNil(t, dm.ValidationError)
		assert.Contains(t, dm.ValidationError.Message, "metadata.json could not be parsed as JSON")
	})

	t.Run("schema failure keeps the parsed document and the declared version", func(t *testing.T) {
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateSBDoor43Metadata(ctx, nil, dm, repo, nil, []byte(`{"format": "scripture burrito", "meta": {"version": "0.3.0"}}`)))
		assert.Equal(t, "sb", dm.MetadataType)
		assert.Equal(t, "0.3.0", dm.MetadataVersion)
		assert.NotNil(t, dm.Metadata)
		require.NotNil(t, dm.ValidationError)
		assert.NotEmpty(t, dm.ValidationError.Causes, "schema violations should carry the per-field causes")
	})

	t.Run("valid document is extracted and clears a previous error", func(t *testing.T) {
		dm := &repo_model.Door43Metadata{ValidationError: metadataFileError("stale")}
		require.NoError(t, populateSBDoor43Metadata(ctx, nil, dm, repo, nil, []byte(validSBMetadata)))
		assert.Nil(t, dm.ValidationError)
		assert.Equal(t, "sb", dm.MetadataType)
		assert.Equal(t, "1.0.0", dm.MetadataVersion)
		assert.Equal(t, "Bible", dm.Subject)
		assert.Equal(t, "Test Bible", dm.Title)
		assert.Equal(t, "en", dm.Language)
		require.Len(t, dm.Ingredients, 1)
		assert.Equal(t, "gen", dm.Ingredients[0].Identifier)
	})
}

// The bundled sb100/common.schema.json widens BCP 47 private-use subtags to 16 characters
// (upstream allows 8) so tags like kvr-x-talangmamak, which DCS uses for languages
// without an ISO code, validate. This guards both that widening and the fact that the
// bundled schema, not the upstream one, is what gets applied.
func TestPopulateSBDoor43Metadata_LongPrivateUseLanguageTag(t *testing.T) {
	ctx := t.Context()
	withTag := func(tag string) []byte {
		doc := strings.ReplaceAll(validSBMetadata, `"tag": "en"`, `"tag": "`+tag+`"`)
		return []byte(strings.ReplaceAll(doc, `"defaultLocale": "en"`, `"defaultLocale": "`+tag+`"`))
	}

	dm := &repo_model.Door43Metadata{}
	require.NoError(t, populateSBDoor43Metadata(ctx, nil, dm, testRepo(1), nil, withTag("kvr-x-talangmamak")))
	assert.Nil(t, dm.ValidationError, "an 11-character private-use subtag must validate")
	assert.Equal(t, "kvr-x-talangmamak", dm.Language)

	dm = &repo_model.Door43Metadata{}
	require.NoError(t, populateSBDoor43Metadata(ctx, nil, dm, testRepo(1), nil, withTag("kvr-x-abcdefghijklmnopq")))
	require.NotNil(t, dm.ValidationError, "a 17-character private-use subtag is still outside the schema's bound")
}

func TestPopulateRCDoor43Metadata(t *testing.T) {
	ctx := t.Context()
	repo := testRepo(1)

	t.Run("unparseable YAML is recorded as an invalid rc entry", func(t *testing.T) {
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateRCDoor43Metadata(ctx, nil, dm, repo, nil, []byte("dublin_core: [unclosed\n  title: x\n")))
		assert.Equal(t, "rc", dm.MetadataType)
		assert.Equal(t, "0.2", dm.MetadataVersion)
		assert.Nil(t, dm.Metadata)
		require.NotNil(t, dm.ValidationError)
		assert.Contains(t, dm.ValidationError.Message, "manifest.yaml could not be parsed as YAML")
	})

	t.Run("schema failure keeps the parsed document and the conformsto version", func(t *testing.T) {
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateRCDoor43Metadata(ctx, nil, dm, repo, nil, []byte("dublin_core:\n  conformsto: rc0.3\n  title: Incomplete\n")))
		assert.Equal(t, "rc", dm.MetadataType)
		assert.Equal(t, "0.3", dm.MetadataVersion)
		assert.NotNil(t, dm.Metadata)
		require.NotNil(t, dm.ValidationError)
		assert.NotEmpty(t, dm.ValidationError.Causes)
	})

	t.Run("valid manifest is extracted", func(t *testing.T) {
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateRCDoor43Metadata(ctx, nil, dm, repo, nil, []byte(validRCManifest)))
		assert.Nil(t, dm.ValidationError)
		assert.Equal(t, "rc", dm.MetadataType)
		assert.Equal(t, "0.2", dm.MetadataVersion)
		assert.Equal(t, "Bible", dm.Subject)
		assert.Equal(t, "Test Bible", dm.Title)
		assert.Equal(t, "Test Publisher", dm.Publisher)
		require.Len(t, dm.Ingredients, 1)
		assert.Equal(t, "gen", dm.Ingredients[0].Identifier)
	})
}

func TestPopulateTcTsDoor43Metadata(t *testing.T) {
	t.Run("unparseable JSON takes its type from the version key it mentions", func(t *testing.T) {
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateTcTsDoor43Metadata(t.Context(), nil, dm, testRepo(1), nil, []byte(`{"package_version": 7,`)))
		assert.Equal(t, "ts", dm.MetadataType)
		assert.Equal(t, "7", dm.MetadataVersion)
		require.NotNil(t, dm.ValidationError)
		assert.Contains(t, dm.ValidationError.Message, "manifest.json could not be parsed as JSON")
	})

	t.Run("repo naming convention outranks the file's hints when the file is invalid", func(t *testing.T) {
		tcRepo := testRepo(1)
		tcRepo.Name = "en_ult_gen_book"
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateTcTsDoor43Metadata(t.Context(), nil, dm, tcRepo, nil, []byte(`{"package_version": 7,`)))
		assert.Equal(t, "tc", dm.MetadataType, "_book suffix means translationCore")
		assert.Equal(t, "8", dm.MetadataVersion)
		require.NotNil(t, dm.ValidationError)

		tsRepo := testRepo(1)
		tsRepo.Name = "fr_gen_text_ulb"
		dm = &repo_model.Door43Metadata{}
		require.ErrorIs(t, populateTcTsDoor43Metadata(t.Context(), nil, dm, tsRepo, nil, []byte(`{"tc_version": 2}`)), errNotTcTsManifest)
		assert.Equal(t, "ts", dm.MetadataType, "_text_ in the name means translationStudio")
		assert.Equal(t, "7", dm.MetadataVersion, "the file's tc_version is not a ts version, so the default applies")
		require.NotNil(t, dm.ValidationError)
	})

	t.Run("wrong field type is recorded, type guessed from the repo name", func(t *testing.T) {
		repo := testRepo(1)
		repo.Name = "en_gen_text_reg"
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateTcTsDoor43Metadata(t.Context(), nil, dm, repo, nil, []byte(`{"project": "gen"}`)))
		assert.Equal(t, "ts", dm.MetadataType)
		assert.NotNil(t, dm.Metadata)
		require.NotNil(t, dm.ValidationError)
		assert.Contains(t, dm.ValidationError.Message, "wrong type")
	})

	t.Run("neither tc nor ts reports errNotTcTsManifest but still describes the entry", func(t *testing.T) {
		dm := &repo_model.Door43Metadata{}
		err := populateTcTsDoor43Metadata(t.Context(), nil, dm, testRepo(1), nil, []byte(`{"package_version": 2, "project": {"id": "gen"}}`))
		require.ErrorIs(t, err, errNotTcTsManifest)
		assert.Equal(t, "ts", dm.MetadataType)
		assert.Equal(t, "2", dm.MetadataVersion)
		assert.NotNil(t, dm.Metadata)
		require.NotNil(t, dm.ValidationError)
		assert.Contains(t, dm.ValidationError.Message, "not a supported manifest")

		dm = &repo_model.Door43Metadata{}
		require.ErrorIs(t, populateTcTsDoor43Metadata(t.Context(), nil, dm, testRepo(1), nil, []byte(`{"name": "some web app"}`)), errNotTcTsManifest)
		assert.Equal(t, "tc", dm.MetadataType, "with nothing to go on the entry defaults to tc")
		assert.Equal(t, "8", dm.MetadataVersion)
		require.NotNil(t, dm.ValidationError)
	})

	t.Run("invalid book id is recorded instead of dropping the entry", func(t *testing.T) {
		dm := &repo_model.Door43Metadata{}
		require.NoError(t, populateTcTsDoor43Metadata(t.Context(), nil, dm, testRepo(1), nil, []byte(`{"tc_version": 8, "project": {"id": "xyz"}}`)))
		assert.Equal(t, "tc", dm.MetadataType)
		assert.Equal(t, "8", dm.MetadataVersion)
		require.NotNil(t, dm.ValidationError)
		assert.Contains(t, dm.ValidationError.Message, `project id "xyz" is not a valid book identifier`)
	})

	t.Run("valid tc manifest is extracted and clears a previous error", func(t *testing.T) {
		dm := &repo_model.Door43Metadata{ValidationError: metadataFileError("stale")}
		require.NoError(t, populateTcTsDoor43Metadata(t.Context(), nil, dm, testRepo(1), nil, []byte(validTcManifest)))
		assert.Nil(t, dm.ValidationError)
		assert.Equal(t, "tc", dm.MetadataType)
		assert.Equal(t, "8", dm.MetadataVersion)
		assert.Equal(t, "Aligned Bible", dm.Subject)
		assert.Equal(t, "en", dm.Language)
		require.Len(t, dm.Ingredients, 1)
		assert.Equal(t, "gen", dm.Ingredients[0].Identifier)
		assert.False(t, dm.Ingredients[0].Exists, "no commit was given, so the book file can't have been found")
	})
}

// commitFileOnBranch writes a one-file tree as a new commit and points refs/heads/branch
// at it, using plumbing so no working tree is needed on the bare fixture repo.
func commitFileOnBranch(t *testing.T, repoPath, branch, fileName, content string) {
	t.Helper()
	commitFilesOnBranch(t, repoPath, branch, map[string]string{fileName: content})
}

// commitFilesOnBranch points branch at a new commit holding only files, keyed by their paths
func commitFilesOnBranch(t *testing.T, repoPath, branch string, files map[string]string) {
	t.Helper()
	ctx := t.Context()

	indexEnv := append(os.Environ(), "GIT_INDEX_FILE="+filepath.Join(t.TempDir(), "index"))
	for fileName, content := range files {
		blobSha, _, err := gitcmd.NewCommand("hash-object", "-w", "--stdin").WithDir(repoPath).WithStdinBytes([]byte(content)).RunStdString(ctx)
		require.NoError(t, err)
		_, _, err = gitcmd.NewCommand("update-index", "--add", "--cacheinfo").AddDynamicArguments("100644," + strings.TrimSpace(blobSha) + "," + fileName).
			WithEnv(indexEnv).WithDir(repoPath).RunStdString(ctx)
		require.NoError(t, err)
	}
	treeSha, _, err := gitcmd.NewCommand("write-tree").WithEnv(indexEnv).WithDir(repoPath).RunStdString(ctx)
	require.NoError(t, err)

	when := time.Now().Format(time.RFC3339)
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_AUTHOR_DATE="+when,
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com", "GIT_COMMITTER_DATE="+when,
	)
	commitSha, _, err := gitcmd.NewCommand("commit-tree").AddDynamicArguments(strings.TrimSpace(treeSha)).
		WithEnv(env).WithDir(repoPath).WithStdinBytes([]byte("add files\n")).RunStdString(ctx)
	require.NoError(t, err)

	_, _, err = gitcmd.NewCommand("update-ref").AddDynamicArguments("refs/heads/"+branch, strings.TrimSpace(commitSha)).WithDir(repoPath).RunStdString(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _, _ = gitcmd.NewCommand("update-ref", "-d").AddDynamicArguments("refs/heads/" + branch).WithDir(repoPath).RunStdString(t.Context())
	})
}

func commitEmptyTreeOnBranch(t *testing.T, repoPath, branch string) {
	t.Helper()
	ctx := t.Context()
	treeSha, _, err := gitcmd.NewCommand("mktree").WithDir(repoPath).WithStdinBytes(nil).RunStdString(ctx)
	require.NoError(t, err)

	when := time.Now().Format(time.RFC3339)
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_AUTHOR_DATE="+when,
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com", "GIT_COMMITTER_DATE="+when,
	)
	commitSha, _, err := gitcmd.NewCommand("commit-tree").AddDynamicArguments(strings.TrimSpace(treeSha)).
		WithEnv(env).WithDir(repoPath).WithStdinBytes([]byte("remove metadata\n")).RunStdString(ctx)
	require.NoError(t, err)

	_, _, err = gitcmd.NewCommand("update-ref").AddDynamicArguments("refs/heads/"+branch, strings.TrimSpace(commitSha)).WithDir(repoPath).RunStdString(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _, _ = gitcmd.NewCommand("update-ref", "-d").AddDynamicArguments("refs/heads/" + branch).WithDir(repoPath).RunStdString(t.Context())
	})
}

// TestProcessDoor43MetadataForRepoRef_InvalidMetadataFiles drives the real pipeline over
// branches whose metadata file exists but is broken. Before this behaviour, an invalid
// metadata.json fell through to the RC path and the whole ref was dropped, so nothing
// ever told the user what was wrong.
func TestProcessDoor43MetadataForRepoRef_InvalidMetadataFiles(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()

	repo, err := repo_model.GetRepositoryByID(ctx, 1)
	require.NoError(t, err)
	repoPath := gitrepo.RepoLocalPath(repo.CodeStorageRepo())

	loadDM := func(t *testing.T, ref string) *repo_model.Door43Metadata {
		t.Helper()
		dm, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref)
		require.NoError(t, err, "the ref must have a door43_metadata entry")
		return dm
	}
	assertInvalid := func(t *testing.T, dm *repo_model.Door43Metadata, metadataType string) {
		t.Helper()
		assert.Equal(t, metadataType, dm.MetadataType)
		require.NotNil(t, dm.ValidationError)
		assert.Equal(t, door43metadata.StageOther, dm.Stage)
		assert.False(t, dm.IsLatestForStage)
		assert.Equal(t, "repo1", dm.Title, "display fields are backfilled from the repo's fallback metadata")
		assert.Equal(t, repo_model.SeverityLevelError, dm.HealthcheckSeverity)
		issues, err := repo_model.GetDoor43HealthcheckIssuesByDMID(ctx, dm.ID)
		require.NoError(t, err)
		require.Len(t, issues, 1, "an invalid file yields only the invalid-metadata finding")
		assert.Equal(t, repo_model.IssueCodeMetadataInvalid, issues[0].IssueCode)
		assert.Equal(t, "META-002", issues[0].Rule)
		assert.Contains(t, issues[0].Suggestion, strings.TrimSuffix(dm.ValidationError.Message, "#"))
	}

	t.Run("sb: unparseable metadata.json", func(t *testing.T) {
		commitFileOnBranch(t, repoPath, "dcs-invalid-sb", "metadata.json", `{"format": "scripture burrito",`)
		require.NoError(t, processDoor43MetadataForRepoRef(ctx, repo, "dcs-invalid-sb"))
		dm := loadDM(t, "dcs-invalid-sb")
		assertInvalid(t, dm, "sb")
		assert.Contains(t, dm.ValidationError.Message, "could not be parsed as JSON")
	})

	t.Run("sb: schema-invalid metadata.json, then fixed", func(t *testing.T) {
		commitFileOnBranch(t, repoPath, "dcs-schema-sb", "metadata.json", `{"format": "scripture burrito", "meta": {"version": "1.0.0"}}`)
		require.NoError(t, processDoor43MetadataForRepoRef(ctx, repo, "dcs-schema-sb"))
		dm := loadDM(t, "dcs-schema-sb")
		assertInvalid(t, dm, "sb")
		assert.NotNil(t, dm.Metadata, "a parseable file is stored even when it fails the schema")

		// The same branch is re-processed once the file is fixed: the existing entry is
		// updated in place and its error cleared.
		commitFileOnBranch(t, repoPath, "dcs-schema-sb", "metadata.json", validSBMetadata)
		require.NoError(t, processDoor43MetadataForRepoRef(ctx, repo, "dcs-schema-sb"))
		fixed := loadDM(t, "dcs-schema-sb")
		assert.Equal(t, dm.ID, fixed.ID)
		assert.Nil(t, fixed.ValidationError)
		assert.Equal(t, "Bible", fixed.Subject)
		assert.Equal(t, "Test Bible", fixed.Title)
		// The declared ingredient doesn't exist in the fixture repo, so deeper checks still
		// find errors; what matters is that the file itself is no longer reported invalid.
		issues, err := repo_model.GetDoor43HealthcheckIssuesByDMID(ctx, fixed.ID)
		require.NoError(t, err)
		for _, issue := range issues {
			assert.NotEqual(t, repo_model.IssueCodeMetadataInvalid, issue.IssueCode)
		}
	})

	t.Run("rc: unparseable manifest.yaml", func(t *testing.T) {
		commitFileOnBranch(t, repoPath, "dcs-invalid-rc", "manifest.yaml", "dublin_core: [unclosed\n  title: x\n")
		require.NoError(t, processDoor43MetadataForRepoRef(ctx, repo, "dcs-invalid-rc"))
		dm := loadDM(t, "dcs-invalid-rc")
		assertInvalid(t, dm, "rc")
		assert.Contains(t, dm.ValidationError.Message, "could not be parsed as YAML")
	})

	t.Run("tc/ts: manifest.json that is neither, with no manifest.yaml", func(t *testing.T) {
		commitFileOnBranch(t, repoPath, "dcs-junk-manifest", "manifest.json", `{"package_version": 2}`)
		require.NoError(t, processDoor43MetadataForRepoRef(ctx, repo, "dcs-junk-manifest"))
		dm := loadDM(t, "dcs-junk-manifest")
		assertInvalid(t, dm, "ts")
		assert.Contains(t, dm.ValidationError.Message, "not a supported manifest")
	})

	t.Run("tc: valid manifest.json", func(t *testing.T) {
		commitFileOnBranch(t, repoPath, "dcs-valid-tc", "manifest.json", validTcManifest)
		require.NoError(t, processDoor43MetadataForRepoRef(ctx, repo, "dcs-valid-tc"))
		dm := loadDM(t, "dcs-valid-tc")
		assert.Nil(t, dm.ValidationError)
		assert.Equal(t, "tc", dm.MetadataType)
		assert.Equal(t, "Aligned Bible", dm.Subject)
	})

	t.Run("a ref with no metadata file is skipped without error", func(t *testing.T) {
		require.NoError(t, processDoor43MetadataForRepoRef(ctx, repo, repo.DefaultBranch))
		_, err := repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, repo.DefaultBranch)
		assert.True(t, repo_model.IsErrDoor43MetadataNotExist(err))
	})

	t.Run("removing metadata deletes the ref entry and its healthcheck issues", func(t *testing.T) {
		const ref = "dcs-metadata-removed"
		commitFileOnBranch(t, repoPath, ref, "metadata.json", validSBMetadata)
		require.NoError(t, processDoor43MetadataForRepoRef(ctx, repo, ref))
		dm := loadDM(t, ref)
		issues, err := repo_model.GetDoor43HealthcheckIssuesByDMID(ctx, dm.ID)
		require.NoError(t, err)
		require.NotEmpty(t, issues, "the initial check must persist issues for the ref")

		commitEmptyTreeOnBranch(t, repoPath, ref)
		require.NoError(t, processDoor43MetadataForRepoRef(ctx, repo, ref))
		_, err = repo_model.GetDoor43MetadataByRepoIDAndRef(ctx, repo.ID, ref)
		require.True(t, repo_model.IsErrDoor43MetadataNotExist(err), "metadata row should be removed")
		issues, err = repo_model.GetDoor43HealthcheckIssuesByDMID(ctx, dm.ID)
		require.NoError(t, err)
		assert.Empty(t, issues, "healthcheck issues should be removed with their metadata row")
	})
}
