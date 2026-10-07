# Release attachment content flags (has_audio / has_video / has_pdf / has_stream / has_other)

## Purpose

Each `door43_metadata` row for a **release** records what kinds of downloadable
content the release offers, based on its attachments. This lets the catalog be
queried for, e.g., all resources that have an audio rendering, without scanning
attachments at query time. All five columns are indexed.

| Column | True when an attachment looks like... |
| ------ | ------------------------------------- |
| `has_audio` | an audio file or an archive of audio files (mp3, m4a, aac, ogg, oga, opus, wav, flac, wma) |
| `has_video` | a video file or an archive of video files (mp4, m4v, mov, avi, mkv, webm, wmv, flv, mpg, mpeg, 3gp) |
| `has_pdf` | a PDF or an archive of PDFs |
| `has_stream` | a link to a streaming platform (YouTube, Vimeo, Dailymotion, Twitch, Rumble, Bilibili, SoundCloud, Spotify, Wistia, Brightcove) |
| `has_other` | anything else (docx, epub, Bloom books, generic zips, ...) — the catch-all |

Branch (non-release) `door43_metadata` rows always have all five flags false.

## How an attachment is classified

`modules/dcs/attachments.go` — `GetAttachmentContentType(name)`.

- The whole attachment name is scanned, including the remote URL of a
  [remote attachment](remote-release-attachments.md) (the raw `name|url`
  encoding). An extension matches when preceded by `.`, `_` or `-`, **anywhere
  in the string** — so `fr_obs_v4.3_mp3_128kbps.zip` (an archive of mp3s)
  counts as audio, not just `*.mp3` files.
- Checks run in order: **audio → video → pdf → stream → other**; the first
  match wins, and each attachment sets exactly one flag.
- Matching is case-insensitive.
- `files.json` / `links.json` manifests are skipped — they are expanded into
  remote attachments and deleted (see
  [remote-release-attachments.md](remote-release-attachments.md)), so they are
  never content themselves.

## When the flags are computed

`Door43Metadata.DetermineAttachmentFlags` (`models/repo/door43metadata.go`)
queries the `attachment` table for the DM's `release_id` and sets the flags. It
runs:

1. **During metadata processing** — `processDoor43MetadataForRepoRef`
   (`services/door43metadata/door43metadata.go`) computes the flags just before
   inserting/updating the DM row. This covers release create/update/delete, the
   `Update Door43 Metadata` cron task, and every other path that reprocesses a
   ref. The metadata notifier expands `files.json` / `links.json` manifests
   **before** processing so the flags see the expanded remote attachments.
2. **On attachment-only changes via the API** — adding, renaming or deleting a
   release asset doesn't reprocess metadata; instead
   `notifyReleaseAttachmentChanged`
   (`routers/api/v1/repo/release_attachment.go`) calls
   `UpdateDoor43MetadataAttachmentFlags`
   (`services/door43metadata/door43metadata.go`), which recomputes and saves
   just the five flag columns. (A manifest attachment change instead re-fires
   the release update notification, which unpacks and fully reprocesses.)

Attachment changes made through the web release editor go through the release
service, which fires the release update notification — path 1.

## Querying the flags

### Catalog API

`/api/v1/catalog/search` and the `/api/v1/catalog/list/*` endpoints (subjects,
metadata-types, owners, languages) accept six optional boolean parameters:

- `hasAudio`, `hasVideo`, `hasPdf`, `hasStream`, `hasOther` — filter on the
  corresponding column; `=1`/`=true` requires the flag, `=0`/`=false` excludes it.
- `hasAttachment` — the abstraction over all five: `=1` matches entries where
  **any** flag is set, `=0` matches entries where **none** are.

They combine with `includeHistory` (now also documented on the list endpoints;
it was already parsed): without it, only the latest entry per repo for the
given stage is considered; with it, **all** entries of the given stage or lower
are. Example — `unfoldingWord/en_obs` v9 (latest prod) has a PDF but no audio,
while v8 has audio and YouTube links:

- `/api/v1/catalog/list/languages?lang=en&subject=Open Bible Stories&hasAudio=1`
  → does **not** include `en` (v9 has no audio)
- `...&hasAudio=1&includeHistory=1` → includes `en` (v8 has audio)
- `...&hasPdf=1` → includes `en` (v9 has a PDF)

List results are `DISTINCT`, so a language/owner/subject/metadata-type appears
once no matter how many entries match. The conditions are built by
`GetContentFlagsCond` in `models/door43metadata/search.go`; scenario coverage is
in `models/catalog_list_test.go`.

### Catalog entry response

Each `CatalogEntry` in API responses carries the flags as an
`attachment_types` object (`modules/structs/catalog.go`,
`services/convert/catalog.go`):

```json
"attachment_types": {
  "pdf": true,
  "audio": true,
  "video": false,
  "stream": true,
  "other": false
}
```

It is always populated for release entries — all-false when the release has no
attachments — and is `null` for branch entries (no `release_id`), which never
have attachments.

### Catalog web page

The `/catalog` page's Search Builder has a `has` dropdown (options:
`attachment`, `audio`, `video`, `pdf`, `stream`, `other`) and an
`include_history` field, which build `has:<type>` / `include_history:true`
tokens in the search query (`routers/web/dcs/catalog.go`,
`templates/shared/searchbuilder.tmpl`). Multiple `has` values are ANDed.

## Schema

The columns are created and indexed automatically by xorm's table sync at
startup (`models/db.SyncAllTables`); no migration is needed. Existing rows get
`false` defaults and are backfilled as releases are reprocessed (the
`Update Door43 Metadata` cron task reprocesses everything).

## Tests

- `modules/dcs/attachments_test.go` — classification of names/URLs.
- `models/repo/door43metadata_test.go` — `TestDetermineAttachmentFlags`
  (fixture + inserted attachments, manifest skipping, branch-ref reset).
