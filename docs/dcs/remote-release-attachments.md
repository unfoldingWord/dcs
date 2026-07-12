# Remote release attachments (files.json / links.json manifests)

## Purpose

DCS lets a release reference **remote** files — a YouTube playlist, a video in
cloud storage, a large archive hosted elsewhere — as if they were normal
release attachments, **without uploading the bytes to DCS**. This avoids storing
very large files while still surfacing them as downloadable assets in the
release and the catalog.

## How to use it

1. Create a JSON manifest describing the remote files. The file name must match
   `(?i)(file|link)s*\.json$` — i.e. `files.json`, `links.json`, `file.json`,
   or `link.json`.
2. Upload that manifest as a **release attachment** (via the web UI or the
   release-attachment API).
3. The Door43 metadata service expands the manifest into one release attachment
   per entry, each pointing at its remote URL, and then **deletes the manifest
   attachment** itself.

### Manifest format

A JSON array of attachment objects (a single object is also accepted):

```json
[
  {
    "name": "YouTube - OBS - Awadhi",
    "size": 0,
    "browser_download_url": "https://www.youtube.com/playlist?list=PLcaGjtXr4D9kq36LRMtXLoFZ3eB9m4Drd"
  }
]
```

- `name` (**required**) — the display name shown for the asset. See the naming
  gotcha below.
- `browser_download_url` (**required**) — the remote URL the asset points to.
- `size` (optional) — byte size; `0`/omitted is fine for links.

## How it works internally

This Gitea version has **no native "external URL" column** on attachments, so a
remote attachment is stored by encoding the URL into the `Name` column as
`name|url`:

- `models/repo/attachment.go` `BeforeInsert`/`BeforeUpdate` write the
  `name|url` encoding.
- `AfterLoad`/`AfterInsert`/`AfterUpdate` split it back into `Name` +
  `BrowserDownloadURL`.
- `DownloadURL()` returns `BrowserDownloadURL` directly when set, so the asset's
  download link is the remote URL.

The expansion itself lives in
`services/door43metadata/door43metadata.go`:

- `UnpackJSONAttachments(ctx, release)` — finds manifest attachments, expands
  each entry into a release attachment, and deletes the manifest.
- `GetAttachmentsFromJSON(attachment)` — fetches the manifest over HTTP and
  unmarshals it (array first, then single-object fallback).

It is triggered from the metadata notifier on release create/update. Because
upstream attachment endpoints don't dispatch a release notification,
`routers/api/v1/repo/release_attachment.go` re-fires the release update
notification when a manifest attachment is added or edited via the API
(`notifyReleaseJSONAttachmentChanged`).

## jsonv2 — struct fields must be tagged

The build sets `GOEXPERIMENT=jsonv2` (see the `Makefile`), so the default
`modules/json` unmarshaler matches JSON member names **case-sensitively**. An
untagged exported Go field only binds an exactly-cased key, so `Name` binds
`"Name"` but **not** `"name"`. The `Attachment` struct therefore carries
explicit `json:"name"` and `json:"size"` tags; without them the manifest's
lowercase keys silently fail to bind. (This is the actual root cause of the
"playlist" name described below — `Name` did not bind, so `BeforeInsert` fell
back to the URL path.)

## Naming gotcha — always supply `name`

If an entry has no bound `name` (omitted, or — before the tag fix above — not
bound at all), `BeforeInsert` falls back to `path.Base(url.Path)`. For URLs
whose meaningful identifier is in the query string this produces a poor name:
for a YouTube playlist `https://www.youtube.com/playlist?list=...` the path is
`/playlist`, so the asset is named **`playlist`**. Always provide an explicit
`name`.

## Behavior notes & limitations

- **Matching is by name.** On re-expansion, an entry updates an existing
  attachment only if the `name` matches. Renaming an entry creates a *new*
  attachment and leaves the old one behind.
- The manifest must be **publicly fetchable by the DCS server**; the fetch has a
  short timeout (`GetAttachmentsFromJSON`).
- The manifest attachment is **deleted** after a successful expansion, so the
  expanded link assets are the source of truth afterward.

## Tests

See `models/repo/attachment_test.go`:

- `TestAttachment_ExternalURLEncoding` — name + URL round-trips correctly.
- `TestAttachment_ExternalURLFallbackName` — documents the `playlist` fallback
  when `name` is omitted.
- `TestAttachment_ManifestUnmarshal` — the manifest shape parses as expected.
