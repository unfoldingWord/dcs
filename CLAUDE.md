# DCS (Door43 Content Service) - Gitea Fork

## Project Overview

DCS is a fork of [Gitea](https://github.com/go-gitea/gitea) that adds Digital Content Service functionality for Bible translation content management. It adds metadata management (Door43Metadata), catalog search, healthcheck dashboards, Scripture Burrito support, and content validation.

### Remotes
- `origin` - `git@github.com:unfoldingWord/dcs.git` (this fork)
- `upstream` - `https://github.com/go-gitea/gitea.git` (Gitea)

### Branch Strategy
- `dcs-base-v1.25` - Primary DCS customization branch (base for all DCS changes)
- `release/dcs/v1.25` - Production release = `upstream/release/v1.25` + `dcs-base-v1.25`
- `main` - Gitea's main + DCS customizations (for tracking upstream main)
- Changes go to `dcs-base-v1.25` first, then merge into `release/dcs/v1.25` and `main`

### Build
```bash
make build                    # Build the binary
make test                     # Run all tests
TAGS="bindata sqlite sqlite_unlock_notify sqlite_json" make test  # With all tags
make test-dcs-sqlite          # DCS-specific integration tests
```

---

## Fork Architecture

### Three categories of files:

1. **New DCS files** (~147 files, zero conflict risk) - Entirely new code in DCS-specific directories
2. **Modified Gitea files** (~127 files, conflict-prone) - Existing Gitea files with inline DCS additions
3. **Upstream-only files** - Unchanged from upstream Gitea

### Key DCS Packages
| Package | Purpose |
|---------|---------|
| `modules/dcs/` | Core DCS utilities: books, languages, subjects, metadata, validation, format handlers (RC02, SB100, TCTS) |
| `modules/setting/dcs.go` | DCS configuration (Door43PreviewURL) |
| `modules/markup/tsv/` | TSV markup renderer |
| `modules/structs/catalog.go` | CatalogEntry, Ingredient, Relation, CatalogStages API structs |
| `models/door43metadata/` | Door43Metadata search and stage models |
| `models/repo/door43metadata.go` | Door43Metadata ORM model (30+ fields) |
| `models/repo/door43healthcheck.go` | Healthcheck model and queries |
| `models/repo/repo_dcs.go` | LoadLatestDMs(), DCS methods on Repository |
| `routers/api/v1/dcs/` | DCS API endpoints (languages, route registration) |
| `routers/api/v1/catalog/` | Catalog API: search, list, entries, metadata, validation |
| `routers/web/dcs/` | DCS web pages: about, catalog, healthcheck dashboard, route registration |
| `services/door43metadata/` | Door43Metadata processing, notifier |
| `services/door43healthcheck/` | Healthcheck service |
| `services/convert/repository_dcs.go` | ToRepoDCS() - populates DCS fields on API repo response |
| `services/convert/catalog.go` | ToCatalogEntry(), ToIngredient(), etc. |
| `services/cron/tasks_dcs.go` | DCS cron tasks (metadata update, user metadata, schema load) |
| `templates/dcs/` | DCS template partials (extracted from inline blocks) |
| `templates/catalog/` | Catalog page templates |
| `templates/repo/dcs_*.tmpl` | DCS repo metadata display templates |
| `web_src/css/dcs.css` | DCS-specific styles |
| `web_src/js/features/dcs-*.ts` | DCS JS features (info icons, language fonts, validation badges) |

---

## DCS Modification Patterns

Every modification to an existing Gitea file follows one of these patterns. When merging upstream changes, apply the pattern-specific merge rule.

### Pattern: STRUCT_FIELD
**What:** DCS fields added to existing Go structs, always at the end with block delimiters.
**Merge rule:** Keep upstream struct changes, re-add DCS fields at end with markers.

```go
// In models/repo/repo.go - Repository struct:
    ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT 0"`
    /*** DCS Customizations ***/
    LatestProdDM    *Door43Metadata `xorm:"-"`
    LatestPreprodDM *Door43Metadata `xorm:"-"`
    DefaultBranchDM *Door43Metadata `xorm:"-"`
    RepoDM          *Door43Metadata `xorm:"-"`
    /*** END DCS Customizations ***/
}
```

### Pattern: ROUTE_BLOCK
**What:** DCS route registrations, extracted into RegisterDCS*() functions.
**Merge rule:** If upstream restructures routing, move the RegisterDCS*() call to the equivalent scope.

```go
// In routers/api/v1/api.go:
    /*** DCS Customizations ***/
    dcs.RegisterDCSAPIRoutes(m, repoAssignment, reqRepoReader)
    /*** END DCS Customizations ***/
```

### Pattern: FUNC_CALL
**What:** Single DCS function call added inside an existing Gitea function.
**Merge rule:** Verify the function still exists and has compatible signature. Re-add call at appropriate point.

```go
// In routers/web/explore/repo.go - inside a handler:
    /*** DCS Customizations ***/
    repos.LoadLatestDMs(ctx)
    /*** END DCS Customizations ***/
```

### Pattern: LOGIC_MOD
**What:** Logic changes within existing Gitea functions (if/else blocks, query modifications). **Highest conflict risk.**
**Merge rule:** Carefully understand both upstream and DCS changes. These require manual integration.

Key files with LOGIC_MOD changes (review carefully during merges):
- `models/repo/repo_list.go` - Search conditions with door43_metadata JOINs
- `models/user/search.go` - User search with door43_metadata filtering
- `routers/web/explore/repo.go` - DCS keyword parsing in search
- `routers/api/v1/repo/repo.go` - API search extensions
- `services/context/repo.go` - Door43Metadata loading in repo assignment

### Pattern: VALUE_CHANGE
**What:** Single-line value changes.
**Merge rule:** May be silently overwritten during merge. Verify line-by-line.

```go
// In routers/api/v1/admin/user.go:
    MustChangePassword: false, // DCS Customizations (was: true)
```

### Pattern: COMMENT_OUT
**What:** Upstream code commented out.
**Merge rule:** If upstream modifies the commented-out code, re-apply the comment-out.

### Pattern: IMPORT
**What:** Additional import statements.
**Merge rule:** Re-add imports if removed during merge. Rarely conflicts.

### Pattern: INTERFACE_EXT
**What:** New methods added to existing Gitea interfaces.
**Merge rule:** Re-add to interface AND all implementors. Check if upstream added new implementors.

```go
// In services/notify/notifier.go:
    /*** DCS Customizations ***/
    NewTagRelease(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, tag string, sha string)
    /*** END DCS Customizations ***/
```

---

## Comment Marker Standard

All DCS modifications to existing Gitea files MUST be marked with comments.

### Multi-line blocks (use opening AND closing):
- **Go:** `/*** DCS Customizations ***/` ... `/*** END DCS Customizations ***/`
- **Templates:** `<!-- DCS Customizations -->` ... `<!-- END DCS Customizations -->`
- **CSS:** `/*** DCS Customizations ***/` ... `/*** END DCS Customizations ***/`
- **JS/TS:** `/** DCS Customizations **/` ... `/** END DCS Customizations **/`
- **INI:** `;;; DCS Customizations` ... `;;; END DCS Customizations`

### Single-line annotations (no END needed):
- **Go:** `// DCS Customizations`

### Important:
- Spelling is `Customizations` (plural, with a z). Fix any typos encountered.
- Every DCS file that is entirely new (not modifying upstream) does NOT need markers.
- The `_dcs.go` suffix convention identifies DCS companion files.

---

## Naming Conventions

| Type | Convention | Example |
|------|-----------|---------|
| Go companion files | `{original}_dcs.go` | `models/repo/repo_dcs.go` |
| DCS-only packages | Dedicated directory | `modules/dcs/`, `models/door43metadata/` |
| DCS route registration | `routes.go` in dcs package | `routers/api/v1/dcs/routes.go` |
| DCS templates (partials) | `templates/dcs/*.tmpl` | `templates/dcs/repo_header_badges.tmpl` |
| DCS templates (pages) | `templates/catalog/*.tmpl` | `templates/catalog/catalog.tmpl` |
| DCS JS features | `dcs-*.ts` | `web_src/js/features/dcs-info-icons.ts` |
| DCS CSS | `dcs.css` | `web_src/css/dcs.css` |
| DCS assets | `public/assets/img/dcs/` | `public/assets/img/dcs/uw.png` |

---

## Merge Procedure Checklist

When merging upstream changes (e.g., new Gitea release):

### Before merge:
1. Ensure `dcs-base-v1.25` is clean and tested
2. `git fetch upstream`
3. Review upstream changelog for breaking changes

### Merge:
```bash
git checkout dcs-base-v1.25
git merge upstream/release/v1.25
```

### Resolve conflicts using pattern rules:
1. For each conflicted file, check `dcs-customizations.md` for the pattern type
2. Apply the merge rule for that pattern (see Modification Patterns above)
3. For STRUCT_FIELD: keep upstream changes, ensure DCS fields remain at end
4. For ROUTE_BLOCK: re-add RegisterDCS*() call in correct scope
5. For FUNC_CALL: verify function signature compatibility, re-add call
6. For LOGIC_MOD: manually integrate both sets of changes
7. For template INJECT: re-add `{{template "dcs/..."}}` at appropriate point

### After merge:
1. `make build` - must compile
2. `make test` - all tests pass
3. Verify DCS markers: `grep -r "DCS Customiz" --include="*.go" --include="*.tmpl" | wc -l`
4. Test key DCS features: catalog, healthcheck, repo metadata display
5. Merge into `release/dcs/v1.25`: `git checkout release/dcs/v1.25 && git merge dcs-base-v1.25`

### If function signatures changed upstream:
1. Check `services/convert/repository_dcs.go` - ToRepoDCS() may need parameter updates
2. Check `models/repo/repo_dcs.go` - LoadLatestDMs() may need updates
3. Check `routers/api/v1/dcs/routes.go` - middleware function signatures may change
4. Check all FUNC_CALL sites in `dcs-customizations.md`

---

## Template Architecture

DCS template modifications use two approaches:

1. **DCS Partials** (`templates/dcs/*.tmpl`) - Extracted blocks included via `{{template "dcs/..." .}}`. Reduces inline modifications to a single line. Preferred approach.

2. **Full replacements** - Some templates are entirely DCS content (e.g., `templates/home.tmpl`). These are documented as FULL_REPLACE in `dcs-customizations.md`. Upstream changes to these files should be evaluated but likely don't apply.

3. **Gitea extension points** (`templates/custom/*.tmpl`) - Gitea's built-in extension system. Used for navbar links, extra tabs, footer scripts. These never conflict with upstream.

DCS template helper functions are registered in `modules/templates/helper.go` and implemented in `modules/dcs/`.
