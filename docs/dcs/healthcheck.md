# Health checks for Door43 resources

## Purpose

Every `door43_metadata` entry (each branch and tag of a resource repo) carries its own
health check result, so the catalog can answer "which resources are actually usable?"
independently of whether someone published a release or added a `tc-ready` topic.

## When checks run

A health check runs for a ref's entry whenever that entry is (re)processed by the
Door43Metadata pipeline (`services/door43metadata`):

- a push to a branch (checks that branch's entry)
- a release or tag is created, updated or tag-ified (checks that tag's entry at its commit)
- repo create/migrate/transfer/fork/rename and default-branch changes (all refs)
- the `update_metadata` cron task (every 72h, all repos), which also serves as the
  backfill after deploying this feature
- the repo's *Update Metadata* web action and the `door43metadata` CLI command

The default-branch entry is checked once more at the end of repo processing so its
"release needed" advice sees the latest prod entry.

## What is checked, by metadata type

Checks implement the DCS Resource Validation Specification where marked with a rule ID
(each stored issue carries the rule in its `rule` field). Severity principle: a repo that
contradicts its own declaration gets **Errors**; deviations from convention and advisory
data (like relations) get **Warnings**; noteworthy-but-legitimate states get **Info**.

Common checks (all of `rc`, `ts`, `tc`, `sb`):

- metadata file is schema-valid — META-002/011 (`validation_error` empty; rc and sb only)
- title is set and does not still say "unfoldingWord" (skipped for the unfoldingWord org)
- language is not left as English `en` (warning; skipped for `en_*` repos)
- every ingredient/project file exists and is non-empty (FILE-001); project titles are translated

`rc` and `sb`:

- repo name's `{lang}_` prefix matches the metadata language — META-019 (warning)

`rc` only:

- publisher changed from "unfoldingWord"; identifier valid for the subject
- relations are **advisory** (nothing depends on their accuracy), so all relation
  findings are warnings or info: a relation whose language differs from the
  resource's (except `hbo`/`el-x-koine`) is a **warning** (REL-004); a relation that
  doesn't resolve to a catalog entry — same owner first as `{lang}_{identifier}`, then
  the fallback owners unfoldingWord and Door43-Catalog — is a **warning** (REL-001,
  REL-002 for `?v=` pins), and one that only resolves under a fallback owner is
  **info**; a resolved Bible/TSV relation missing this resource's books is a warning
- TSV Translation Notes should have `tw`, `ta`, `glt`, `gst` relations (warning)
- Open Bible Stories: a missing story is a **warning** (COMP-020 — OBS can be healthy
  while incomplete); an existing story with no title or no frames is an **error**
  (MD-002); a missing final Bible-reference line is a warning

`rc`/`sb`/`tc` scripture subjects (Bible, Aligned Bible, Greek NT, Hebrew OT):

- every `.usfm` book ingredient is structurally valid: leading `\id` matching the
  declared book (USFM-001/002) and `\c`/`\v` markers present (USFM-004/005) — errors
- Aligned Bible books with no `\zaln-s` alignment data — **warning** for now (USFM-009)
- for `tc` repos, a valid `manifest.json` and the root `{repo_name}.usfm` are the only
  things that matter for release; everything under `.apps/` and the per-chapter JSON
  dirs is tC-internal working data and is deliberately never validated (STRUCT-011
  was removed from the spec)

`rc`/`sb` TSV subjects (TSV-001…011):

- header row exactly matches the subject's column schema (legacy 9-column TN files are
  tolerated); every row has the header's column count; `ID` matches `^[a-z][a-z0-9]{3}$`
  and is unique per file (duplicates break tC-Create); `Reference` grammar (incl.
  `{c}:front` and compound verse lists like `5:1,3,8-12`); `Occurrence` is an integer
  ≥ -1 when Quote/OrigWords has text (0 or blank when it's empty, e.g. intro rows);
  `TWLink`/`SupportReference` match the rc:// grammar; and required cells are non-empty
  (`Note` for notes, `Question`/`Response` for questions, `OrigWords`/`TWLink` for word
  links) — all errors. Findings list row numbers, capped at 10 per finding.

`sb` only:

- every entry in metadata.json's `ingredients` exists in the repo at that commit —
  **error** (FILE-001); size mismatches are **warnings** (META-015), and MD5 checksums
  are compared (as warnings) only when validating a tag/release (D6)

"Release needed" advice (default branch only) is **Info** severity: publishing is never
gated on health; the catalog just reports each entry's own severity.

## Storage

- `door43_metadata.healthcheck_severity` — overall severity (indexed):
  1=success, 2=info, 3=warning, 4=error; NULL/0 = never checked
- `door43_metadata.healthcheck_counts` — JSON counts per severity
- `door43_metadata.healthcheck_time_unix` — when the check last ran
- `door43_healthcheck_issue` — one row per issue (code, severity, titles, details,
  suggestion), replaced on each run, deleted with its entry

Read paths (API endpoint, repo health check page, badge tooltips) serve the stored
rows; the check is only re-run on read when an entry was never checked or its stored
rows predate issue persistence.

### Deleted refs

A branch or tag can be deleted while its check is still running. Three defenses keep
results from outliving their ref:

- `StoreHealthcheckResults` writes severity, counts, time and issues in one transaction
  that first verifies the entry still exists (`SELECT ... FOR UPDATE` on MySQL; SQLite
  serializes writers) — a check that finishes after the delete discards its results.
- The delete helpers remove the entry row and its issues in one transaction, entry row
  first, so a concurrent store fully serializes against the delete.
- The all-refs pass (`update_metadata` cron, repo events) sweeps entries whose ref no
  longer exists (`DeleteDoor43MetadatasStaleRefs`) — the backstop for the remaining
  window where a ref-deletion notification lands before an in-flight insert. The sweep
  only runs when the tag and branch listings both succeeded, and spares rows touched
  after the pass started (e.g. a branch pushed mid-pass).

## Querying

- `is_healthy=true|false` on `/api/v1/catalog/search`, `/api/v1/catalog/list/*`,
  `/api/v1/catalog/stats` and `/stats-ext`: healthy = severity success/info/**warning**
  (warnings ignored by default). `false` returns errored **and never-checked** entries.
- `is_healthy_without_warnings=true|false`: the strict variant (success/info only).
- `healthcheck=error,warning,info,success` (comma list) on the same endpoints matches
  entries by exact overall severity — the general form when the two booleans aren't
  enough. Never-checked entries match no level.
- Catalog entries include `healthcheck_severity`, `is_healthy`,
  `is_healthy_without_warnings` and `healthcheck_url`.
- `/stats-ext` includes `is_healthy_count` and `is_healthy_without_warnings_count`.
- The web catalog Search Builder supports `is_healthy:` and
  `is_healthy_without_warnings:` tokens; repo search keeps the
  `healthcheck:severity` filter.
- Full results: `GET /api/v1/repos/{owner}/{repo}/healthcheck?ref={branch|tag}`.
- Web: `/{owner}/{repo}/healthcheck` shows the repo's canonical entry;
  `/{owner}/{repo}/healthcheck/{ref}` shows a specific branch or tag. The badges on
  the branches/tags/releases pages (and everywhere the shared badge partial is used
  with a ref) link to the ref-specific page.

Implementation: `services/door43healthcheck/` (checks per type/format in `checks_*.go`),
`models/repo/door43healthcheck.go` (issue codes, rules, messages, persistence),
`models/door43metadata/search.go` (`GetIsHealthyCond`, `GetHealthcheckCond`). Tests:
`models/repo/door43healthcheck_test.go`, `models/catalog_list_test.go`
(`TestSearchCatalogIsHealthyFilter`), `services/door43healthcheck/checks_tsv_test.go`,
`checks_usfm_test.go`.
