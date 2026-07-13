# Catalog stats endpoints (/api/v1/catalog/stats and /stats-ext)

## Purpose

Aggregate counts over the catalog in a single request, so clients (dashboards,
reports) don't have to page through `/catalog/search` and count client-side.

## Endpoints

- `GET /api/v1/catalog/stats` — returns the counts object below.
- `GET /api/v1/catalog/stats-ext` — same counts plus the sorted unique values
  of subjects, flavor types, flavors, owners, languages and metadata types.

Both accept **all the regular catalog search filters** — `q`, `owner`, `repo`,
`tag`, `lang`, `is_gl`, `stage`, `subject`, `flavorType`, `flavor`,
`abbreviation`, `format`, `checkingLevel`, `book`, `metadataType`,
`metadataVersion`, `topic`, `withoutTopic`, the `has*` content flags,
`includeHistory` and `partialMatch` — plus two date bounds:

- `startDate` — only entries with `release_date_unix` **on or after** this date.
- `endDate` — only entries **on or before** this date. A date given without a
  time (e.g. `2026-07-13`) is inclusive of that whole day.

Dates are parsed in UTC by `ParseHumanDate` (`modules/dcs/datetime.go`), which
accepts a Unix timestamp or common formats like `2026-07-13`,
`2026-07-13 15:04:05`, `2026-07-13T15:04:05Z`, `Jul 13, 2026`, `07/13/2026`.

## Response shape

```json
{
  "entry_count": 120,
  "lang_count": 35,
  "lang_ltr_count": 30,
  "lang_rtl_count": 5,
  "subject_count": 12,
  "flavor_type_count": 4,
  "flavor_count": 7,
  "owner_count": 20,
  "repo_count": 110,
  "ts_count": 0,
  "tc_count": 2,
  "rc_count": 80,
  "sb_count": 38,
  "has_pdf": 40,
  "has_audio": 15,
  "has_video": 5,
  "has_stream": 12,
  "has_other": 30,
  "has_attachment": 102
}
```

`/stats-ext` adds the healthcheck counts and the unique value lists:

```json
{
  "healthcheck_success_count": 50,
  "healthcheck_info_count": 10,
  "healthcheck_warning_count": 5,
  "healthcheck_error_count": 2,
  "no_healthcheck_count": 53,
  "subjects": ["Bible", "Open Bible Stories"],
  "flavor_types": ["gloss", "scripture"],
  "flavors": ["textStories", "textTranslation"],
  "owners": ["door43-catalog", "unfoldingword"],
  "languages": ["ar", "en", "fr"],
  "metadata_types": ["rc", "sb"]
}
```

## Semantics

- `entry_count` — matching `door43_metadata` rows. Without `includeHistory`,
  that's the latest entry per repo for the stage; with it, every release of the
  given stage or lower.
- `lang_count`, `subject_count`, `flavor_type_count`, `flavor_count`,
  `owner_count`, `repo_count` — `DISTINCT` counts of the respective field.
- `lang_ltr_count` / `lang_rtl_count` — **unique languages** whose
  `language_direction` is `ltr` / `rtl` (so they partition `lang_count`).
- `ts_count`/`tc_count`/`rc_count`/`sb_count`, `has_*` and `healthcheck_*` —
  **entry counts** (rows matching that property). With the default
  no-history/prod query these equal per-repo counts, since each repo
  contributes one entry.
- `has_attachment` — the sum of the five `has_*` counts, so an entry with,
  say, both audio and PDF contributes twice.
- The `healthcheck_*_count` fields are only in `/stats-ext`.
  `no_healthcheck_count` counts entries whose `healthcheck_severity` is NULL
  or 0. Healthchecks only run against default-branch metadata, so
  release-stage queries report mostly "none".
- The `/stats-ext` value lists are sorted ascending; owners are lowercase
  (`user.lower_name`), making the sort case-insensitive.

Implementation: `GetCatalogStats` / `GetCatalogStatsExt` in
`models/catalog_list.go` (one aggregate query reusing `SearchCatalogCondition`,
plus one `DISTINCT` query per list for `/stats-ext`); handlers in
`routers/api/v1/catalog/catalog.go`. Tests: `models/catalog_list_test.go`
(`TestGetCatalogStats`) and `modules/dcs/datetime_test.go`.
