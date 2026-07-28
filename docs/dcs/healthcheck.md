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

Common checks (all of `rc`, `ts`, `tc`, `sb`):

- metadata file is schema-valid (`validation_error` empty; rc and sb only — ts/tc have no schema)
- title is set and does not still say "unfoldingWord" (skipped for the unfoldingWord org)
- language is not left as English `en` (warning; skipped for `en_*` repos)
- every ingredient/project file exists and is non-empty; project titles are translated

`rc` only:

- publisher changed from "unfoldingWord"; identifier valid for the subject
- relations use the resource's language; TSV Translation Notes have `tw`, `ta`, `glt`, `gst` relations
- each relation resolves to a catalog entry (checked against the `door43_metadata`
  table directly, not over HTTP) and, for Bible/TSV subjects, contains this
  resource's books
- Open Bible Stories: all 50 stories exist with titles, frames and Bible references

`tc` only:

- the USFM file is structurally valid: leading `\id` marker matching the manifest's
  book, and chapter/verse markers present (error)
- the USFM contains alignment data (`\zaln-s`) — **warning** for now

`sb` only:

- every entry in metadata.json's `ingredients` matches the repo at that commit:
  the path exists, the size matches, and the MD5 checksum (when given) matches

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

## Querying

- `is_healthy=true|false` on `/api/v1/catalog/search`, `/api/v1/catalog/list/*`,
  `/api/v1/catalog/stats` and `/stats-ext`: healthy = severity success/info/**warning**
  (warnings ignored by default). `false` returns errored **and never-checked** entries.
- `is_healthy_without_warnings=true|false`: the strict variant (success/info only).
- Catalog entries include `healthcheck_severity`, `is_healthy`,
  `is_healthy_without_warnings` and `healthcheck_url`.
- `/stats-ext` includes `is_healthy_count` and `is_healthy_without_warnings_count`.
- The web catalog Search Builder supports `is_healthy:` and
  `is_healthy_without_warnings:` tokens; repo search keeps the
  `healthcheck:severity` filter.
- Full results: `GET /api/v1/repos/{owner}/{repo}/healthcheck?ref={branch|tag}`.

Implementation: `services/door43healthcheck/` (checks per type in `checks_*.go`),
`models/repo/door43healthcheck.go` (issue codes, messages, persistence),
`models/door43metadata/search.go` (`GetIsHealthyCond`). Tests:
`models/repo/door43healthcheck_test.go`, `models/catalog_list_test.go`
(`TestSearchCatalogIsHealthyFilter`), `services/door43healthcheck/checks_tc_test.go`.
