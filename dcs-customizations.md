---
upstream_base: v1.25
total_modified_files: 127
total_new_files: 147
last_updated: 2026-03-12
---

# DCS Customizations Registry

This document catalogs every DCS modification to existing Gitea files. New DCS-only files are listed at the end. Use this during upstream merges to identify what needs attention.

## Modified Go Files

### STRUCT_FIELD - Fields added to existing structs

| File | Description | Risk | Depends On |
| ------ | ------------- | ------ | ------------ |
| `models/repo/repo.go` | LatestProdDM, LatestPreprodDM, DefaultBranchDM, RepoDM on Repository | HIGH | models/repo/door43metadata.go |
| `models/user/user.go` | RepoLanguages, RepoSubjects, RepoMetadataTypes on User (JSON TEXT columns) | MEDIUM | - |
| `modules/structs/repo.go` | 18 DCS fields (MetadataType through HealthcheckURL) on Repository API struct | MEDIUM | modules/structs/catalog.go |
| `modules/structs/org.go` | RepoLanguages, RepoSubjects, RepoMetadataTypes on Organization | MEDIUM | - |
| `modules/structs/user.go` | RepoLanguages, RepoSubjects, RepoMetadataTypes on User API struct | MEDIUM | - |
| `modules/structs/pull.go` | Status, ConflictedFiles on PullRequest | LOW | - |
| `modules/structs/release.go` | Door43Metadata (*CatalogEntry) on Release | LOW | modules/structs/catalog.go |
| `services/context/repo.go` | Door43Metadata on context.Repository | HIGH | models/repo/door43metadata.go |
| `services/gitdiff/gitdiff.go` | Entry (*git.TreeEntry) on DiffFile | LOW | - |

### ROUTE_BLOCK - Route registrations

| File | Description | Risk | Depends On |
| ------ | ------------- | ------ | ------------ |
| `routers/api/v1/api.go` | 4 blocks: reqExploreSignIn disabled, healthcheck endpoint, spam admin, catalog/languages routes | HIGH | routers/api/v1/dcs/, routers/api/v1/catalog/ |
| `routers/web/web.go` | 2 blocks: repo healthcheck/metadata routes, top-level about/tools/catalog | HIGH | routers/web/dcs/, routers/web/repo/door43metadata.go |

### FUNC_CALL - Single function calls added to existing functions

| File | Description | Risk | Depends On |
| ------ | ------------- | ------ | ------------ |
| `models/repo/repo.go` | LoadLatestDMs() call in LoadAttributes() | HIGH | models/repo/repo_dcs.go |
| `modules/setting/setting.go` | loadDCSFrom(cfg) call | LOW | modules/setting/dcs.go |
| `modules/templates/helper.go` | 13 DCS template functions registered | MEDIUM | modules/dcs/ |
| `routers/init.go` | mustInitCtx(ctx, door43metadata.Init) | MEDIUM | services/door43metadata/ |
| `routers/private/default_branch.go` | ProcessDoor43MetadataForRepo() call | MEDIUM | services/door43metadata/ |
| `routers/api/v1/repo/release_attachment.go` | notifyReleaseAttachmentChanged() helper + calls in create/edit/delete handlers (manifest expansion, has_* flag updates) | MEDIUM | services/door43metadata/, modules/dcs/attachments.go |
| `routers/web/repo/view_file.go` | FileExt, IgnoreLanguageDirection ctx.Data | LOW | - |
| `routers/web/repo/view_home.go` | IgnoreLanguageDirection, Entry ctx.Data | LOW | - |
| `routers/web/repo/editor.go` | Entry ctx.Data set | LOW | - |
| `routers/web/repo/blame.go` | Entry ctx.Data set | LOW | - |
| `routers/web/user/home.go` | repos.LoadLatestDMs(ctx) in Milestones | LOW | models/repo/repo_dcs.go |
| `routers/web/user/notification.go` | repos.LoadLatestDMs(ctx) | LOW | models/repo/repo_dcs.go |
| `routers/web/admin/orgs.go` | RepoLanguages filter | LOW | - |
| `services/cron/tasks_basic.go` | 3 DCS cron task registrations | LOW | services/cron/tasks_dcs.go |
| `services/convert/convert.go` | RepoLanguages/Subjects/MetadataTypes on Org | LOW | - |
| `services/convert/pull.go` | Status, ConflictedFiles mapping | LOW | - |
| `services/convert/release.go` | ToCatalogEntry() for Door43Metadata | LOW | services/convert/catalog.go |
| `services/convert/user.go` | RepoLanguages/Subjects/MetadataTypes mapping | LOW | - |

### LOGIC_MOD - Logic changes in existing functions (highest conflict risk)

| File | Description | Risk | Depends On |
| ------ | ------------- | ------ | ------------ |
| `models/repo/repo_list.go` | 33 DCS mods: search options, door43_metadata JOINs, column prefixing | HIGH | models/door43metadata/ |
| `models/user/search.go` | DCS search fields, door43_metadata subquery JOINs (~65 lines) | HIGH | models/door43metadata/ |
| `models/repo/release.go` | Door43Metadata field, InCatalog filter, GetLatestReleaseByRepoID signature change | HIGH | models/repo/door43metadata.go |
| `models/repo/attachment.go` | BrowserDownloadURL, XORM hooks for URL encoding | MEDIUM | - |
| `models/issues/pull.go` | String() method on PullRequestStatus | LOW | - |
| `models/unittest/fixtures.go` | WITH allowed as read-only SQL in fixtures hook (CTE catalog queries) | LOW | models/catalog_list.go |
| `routers/api/v1/repo/repo.go` | 20+ DCS swagger params, search field population | HIGH | models/door43metadata/ |
| `routers/api/v1/repo/release.go` | pre-release, in-catalog params, InCatalog filter | MEDIUM | - |
| `routers/api/v1/repo/git_ref.go` | Refactored getGitRefsInternal, added CRUD handlers | MEDIUM | services/gitref/ |
| `routers/api/v1/org/org.go` | DCS swagger params, filter fields | LOW | - |
| `routers/api/v1/user/user.go` | DCS swagger params, filter fields | LOW | - |
| `routers/api/v1/user/app.go` | Duplicate token names allowed, default scopes | MEDIUM | - |
| `routers/web/explore/repo.go` | DCS keyword parsing, search fields, LoadLatestDMs | HIGH | models/repo/repo_dcs.go |
| `routers/web/user/profile.go` | DCS keyword parsing, search fields, LoadLatestDMs | HIGH | models/repo/repo_dcs.go |
| `routers/web/org/home.go` | DCS keyword parsing, search fields, LoadLatestDMs | HIGH | models/repo/repo_dcs.go |
| `routers/web/admin/users.go` | Spam user filter, LoadLatestDMs | MEDIUM | - |
| `routers/web/repo/release.go` | Door43Metadata loading for releases/tags, GetLatestReleaseByRepoID call | HIGH | models/repo/door43metadata.go |
| `routers/web/repo/repo.go` | DownloadSB/InitiateDownloadSB handlers, GetLatestReleaseByRepoID call | MEDIUM | services/sbarchiver/ |
| `routers/web/repo/compare.go` | TSV reader, TreeEntry attachment for diffs | MEDIUM | - |
| `routers/web/repo/pull.go` | TreeEntry attachment for JSON/YAML validation | MEDIUM | - |
| `services/context/repo.go` | LoadLatestDMs in repoAssignment, Door43Metadata by ref in RepoRefByType | HIGH | models/repo/door43metadata.go |
| `services/convert/repository.go` | Removed Language field, wraps return with ToRepoDCS() | MEDIUM | services/convert/repository_dcs.go |
| `services/release/release.go` | notify_service.NewTagRelease() call in CreateNewTag | MEDIUM | services/notify/notifier.go |
| `services/repository/create.go` | LICENSE → LICENSE.md extension change | LOW | - |
| `services/auth/auth.go` | archivePathRe extended for /sb/ path | LOW | - |
| `services/forms/repo_form.go` | NewDoor43MetadataForm, EditDoor43MetadataForm structs | LOW | - |

### VALUE_CHANGE - Single-line value changes

| File | Description | Risk | Depends On |
| ------ | ------------- | ------ | ------------ |
| `routers/api/v1/admin/user.go` | MustChangePassword: false (was true) | LOW | - |
| `modules/markup/csv/csv.go` | Removes .tsv from csv renderer extensions | LOW | modules/markup/tsv/ |
| `services/repository/create.go` | LICENSE → LICENSE.md | LOW | - |

### COMMENT_OUT - Upstream code commented out

| File | Description | Risk | Depends On |
| ------ | ------------- | ------ | ------------ |
| `routers/api/v1/api.go` | reqExploreSignIn body commented out | LOW | - |
| `routers/common/blockexpensive.go` | Several paths removed/changed | LOW | - |
| `routers/web/user/setting/applications.go` | Token name uniqueness check disabled | LOW | - |

### IMPORT - Additional imports

| File | Description | Risk | Depends On |
| ------ | ------------- | ------ | ------------ |
| `main.go` | `_ "gitea.dev/modules/markup/tsv"` | LOW | modules/markup/tsv/ |
| `cmd/main.go` | CmdDoor43Metadata command registration | LOW | cmd/door43metadata.go |
| `routers/init.go` | door43metadata service import | LOW | services/door43metadata/ |
| `routers/web/web.go` | routers/web/dcs import | LOW | routers/web/dcs/ |
| `go.mod` | github.com/unfoldingWord/go-rc2sb and go-ts2rc dependencies | LOW | - |

### INTERFACE_EXT - Interface extensions

| File | Description | Risk | Depends On |
| ------ | ------------- | ------ | ------------ |
| `services/notify/notifier.go` | NewTagRelease() added to Notifier interface | MEDIUM | All Notifier implementors |
| `services/notify/null.go` | NewTagRelease() stub on NullNotifier | MEDIUM | services/notify/notifier.go |

---

## Modified Template Files

### INJECT - DCS blocks injected into existing templates

| File | Description | Risk |
| ------ | ------------- | ------ |
| `templates/base/head.tmpl` | dcs_testing_banner partial include | LOW |
| `templates/base/footer_content.tmpl` | CC BY-SA 4.0 license notice | LOW |
| `templates/base/head_navbar.tmpl` | Help/API links replaced, usage stats link | MEDIUM |
| `templates/shared/repo/list.tmpl` | Healthcheck badge, catalog badges, metadata display | HIGH |
| `templates/shared/repo/search.tmpl` | Search builder UI include | MEDIUM |
| `templates/repo/header.tmpl` | Catalog version badges, tag button, file icon | MEDIUM |
| `templates/repo/sub_menu.tmpl` | Language, metadata type, repo size display | MEDIUM |
| `templates/repo/view_content.tmpl` | Preview button, validation badge | MEDIUM |
| `templates/repo/view_file.tmpl` | Expand toggle, direction attrs, validation script | MEDIUM |
| `templates/repo/view_list.tmpl` | Validation badge for JSON/YAML | LOW |
| `templates/repo/release/list.tmpl` | USFM script, catalog badges, preview links | HIGH |
| `templates/repo/release/new.tmpl` | BrowserDownloadURL attachment input | LOW |
| `templates/repo/branch/list.tmpl` | USFM script, preview buttons per branch | MEDIUM |
| `templates/repo/tag/list.tmpl` | USFM script, preview buttons per tag | MEDIUM |
| `templates/repo/diff/box.tmpl` | Validation badge in diff headers | LOW |
| `templates/repo/create.tmpl` | CC-BY-SA license override, hidden fields | MEDIUM |
| `templates/repo/clone_panel.tmpl` | Scripture Burrito download links | LOW |
| `templates/repo/home.tmpl` | USFM alignment remover script | LOW |
| `templates/user/auth/signup_inner.tmpl` | Agreement notice, field helper text | MEDIUM |
| `templates/admin/user/list.tmpl` | Spam user filter radio | LOW |
| `templates/admin/dashboard.tmpl` | 3 DCS operation rows | LOW |
| `templates/repo/settings/options.tmpl` | Actions checkbox restricted to admins | LOW |
| `templates/repo/diff/csv_diff.tmpl` | GetCsvCellDiff function call | LOW |
| `templates/repo/home_sidebar_bottom.tmpl` | "File Types" label change | LOW |

### FULL_REPLACE - Completely replaced templates

| File | Description | Risk |
| ------ | ------------- | ------ |
| `templates/home.tmpl` | Entire DCS landing page (replaces Gitea default) | LOW (no upstream content to preserve) |

### TRIVIAL - Minor icon/text changes

| File | Description | Risk |
| ------ | ------------- | ------ |
| `templates/explore/navbar.tmpl` | octicon-code → octicon-file | LOW |
| `templates/repo/editor/edit.tmpl` | octicon-code → octicon-file | LOW |
| `templates/swagger/ui.tmpl` | "Gitea API" → "DCS (Gitea) API" | LOW |

### BRANDING/AUTO-GENERATED

| File | Description | Risk |
| ------ | ------------- | ------ |
| `templates/swagger/v1_json.tmpl` | ~2700 lines of DCS API definitions added | LOW (auto-generated) |

---

## Modified Config/Build Files

| File | Description | Risk |
| ------ | ------------- | ------ |
| `.changelog.yml` | Repo name: unfoldingWord/dcs | LOW |
| `.github/workflows/release-tag-version.yml` | CI runner change | LOW |
| `.gitignore` | DCS-specific entries | LOW |
| `Dockerfile` | sqlite_json tag, DCS tools (jq, yq, nodejs) | MEDIUM |
| `Dockerfile.rootless` | sqlite_json tag | LOW |
| `Makefile` | Docker image name, test targets | MEDIUM |
| `README.md` | DCS branding | LOW |
| `custom/conf/app.example.ini` | [dcs] section | LOW |
| `docker/manifest*.tmpl` | Docker image references | LOW |
| `options/locale/locale_en-US.ini` | DCS locale strings, rebranding | MEDIUM |

---

## Modified Frontend Files

| File | Description | Risk |
| ------ | ------------- | ------ |
| `web_src/css/index.css` | DCS CSS import | LOW |
| `web_src/css/markup/content.css` | Markup style tweaks | LOW |
| `web_src/css/standalone/swagger.css` | Swagger style tweaks | LOW |
| `web_src/js/features/repo-home.ts` | Minor DCS integration | LOW |
| `web_src/js/features/repo-projects.ts` | Minor DCS integration | LOW |
| `web_src/js/index-domready.ts` | DCS feature imports | LOW |

---

## Modified Test Files

| File | Description | Risk |
| ------ | ------------- | ------ |
| `services/auth/auth_test.go` | Test for /sb/ archive path | LOW |
| `services/release/release_test.go` | Changed test targets for DCS compatibility | LOW |
| `tests/integration/links_test.go` | Updated for DCS links | LOW |
| `tests/integration/user_test.go` | Updated for DCS behavior | LOW |

---

## New DCS-Only Files (zero conflict risk)

### Go packages

- `cmd/door43metadata.go`
- `models/catalog_list.go`, `catalog_list_test.go`
- `models/door43metadata/search.go`, `stage.go`
- `models/git/refs.go`
- `models/repo/catalog.go`, `door43healthcheck.go`, `door43metadata.go`, `door43metadata_test.go`, `release_dcs.go`, `repo_dcs.go`
- `models/user_dcs.go`
- `models/fixtures/door43_metadata.yml`
- `modules/dcs/attachments.go`, `books.go`, `datetime.go`, `files.go`, `languages.go`, `metadata.go`, `rc02.go`, `sb100.go`, `stats.go`, `strings.go`, `subjects.go`, `tcts.go`, `valdation.go`
- `modules/markup/tsv/tsv.go`
- `modules/options/dcs.go`
- `modules/setting/dcs.go`
- `modules/structs/catalog.go`, `dcs_tctsmanifest.go`
- `routers/api/v1/admin/user_dcs.go`
- `routers/api/v1/catalog/catalog.go`
- `routers/api/v1/dcs/dcs.go`
- `routers/api/v1/repo/door43healthcheck.go`
- `routers/api/v1/swagger/catalog.go`, `dcs.go`
- `routers/web/dcs/about.go`, `catalog.go`, `healthcheck_dashboard.go`
- `routers/web/repo/door43metadata.go`
- `services/convert/catalog.go`, `catalog_test.go`, `git_ref.go`, `repository_dcs.go`
- `services/cron/tasks_dcs.go`
- `services/door43healthcheck/healthcheck.go`, `obs_checks.go`
- `services/door43metadata/door43metadata.go`, `door43metadata_notifier.go`, `door43metadata_test.go`
- `services/gitref/gitref.go`, `gitref_test.go`

### Templates

- `templates/catalog/catalog.tmpl`, `catalog_list.tmpl`, `catalog_search.tmpl`, `catalog_publisher_list.tmpl`, `hc_dash.tmpl`, `info_icon.tmpl`
- `templates/dcs_testing_banner.tmpl`
- `templates/repo/dcs_metadata.tmpl`, `dcs_metadata_list.tmpl`, `dcs_metadata_list_item.tmpl`, `dcs_healthcheck.tmpl`, `dcs_healthcheck_list.tmpl`
- `templates/shared/healthcheck_badge.tmpl`, `searchbuilder.tmpl`
- `templates/tools.tmpl`

### CI/CD

- `.github/workflows/dcs-tests.yml`, `release-nightly-dcs.yml`, `release-tag-version-dcs.yml`
- `.eslintignore`

### Assets

- `assets/lang_font_families.json`, `lang_font_links.json`
- `options/label/BibleBooks`, `options/license/CC-BY-SA-4.0.md`
- `options/schema/rc02/`, `options/schema/sb100/` (many schema files)
- `public/assets/img/dcs/` (bg.png, rc.png, sb.png, tc.png, tools.jpg, ts.png, uw-32.png, uw.png)
- `public/assets/js-dcs/usfm-alignment-remover.js`

### Frontend

- `web_src/css/dcs.css`
- `web_src/js/features/dcs-info-icons.ts`, `dcs-language-fonts.ts`, `dcs-search-builder.ts`, `dcs-validation-badge.ts`
