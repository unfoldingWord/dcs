# DCS (Door43 Content Service) - Gitea Fork

## Project Overview

DCS is a fork of [Gitea](https://github.com/go-gitea/gitea) that adds Digital Content Service functionality for Bible translation content management. It adds metadata management (Door43Metadata), catalog search, healthcheck dashboards, Scripture Burrito support, and content validation.

### Remotes

- `origin` - `git@github.com:unfoldingWord/dcs.git` (this fork)
- `upstream` - `https://github.com/go-gitea/gitea.git` (Gitea)

### Build

```bash
TAGS="bindata" make build            # Build the binary (sqlite is built-in since v1.27)
make test                     # Run all tests
TAGS="bindata" make test             # With bindata
make test-dcs-sqlite          # DCS-specific integration tests
```

**Important:** After merging upstream changes or switching branches, run `make vendor` (or `go mod tidy && go mod vendor`) to sync the vendor directory with `go.mod` before building. The build will fail if the vendor directory is out of sync.

### Instructions for agents (from AGENTS.md)

- Use `make help` to find available development targets
- Before committing `.go` changes, run `make fmt` to format, and run `make lint-go` to lint
- Before committing `.ts` changes, run `make lint-js` to lint
- Before committing `go.mod` changes, run `make tidy`
- Before committing new `.go` files, add the current year into the copyright header
- Before committing any files, remove all trailing whitespace from source code lines
- Never force-push to pull request branches
- Always start issue and pull request comments with an authorship attribution

See @AGENTS.md for more details which comes from upstream Gitea

### Pre-PR Checklist

Before creating a PR, run these checks to ensure CI will pass:

- `TAGS="bindata" make build` - must compile (sqlite is built-in since v1.27)
- `make lint-spell` - fix any misspellings
- `make lint-go` - fix any Go lint errors
- `make lint-templates` - fix template lint errors
- `make checks-backend` - regenerates `go-licenses.json` and swagger; commit any changes
- `make lint-md` - fix markdown lint errors (blank lines around headings/lists/tables/fences, no trailing punctuation in headings)
- Verify DCS translation keys exist in locale files for any new cron tasks or admin dashboard items (see Localization section)

---

## Fork Architecture

### Three categories of files

1. **New DCS files** (~147 files, zero conflict risk) - Entirely new code in DCS-specific directories
2. **Modified Gitea files** (~127 files, conflict-prone) - Existing Gitea files with inline DCS additions
3. **Upstream-only files** - Unchanged from upstream Gitea

### Key DCS Packages

| Package | Purpose |
| ------- | ------- |
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

### Multi-line blocks (use opening AND closing)

- **Go:** `/*** DCS Customizations ***/` ... `/*** END DCS Customizations ***/`
- **Templates:** `<!-- DCS Customizations -->` ... `<!-- END DCS Customizations -->`
- **CSS:** `/*** DCS Customizations ***/` ... `/*** END DCS Customizations ***/`
- **JS/TS:** `/** DCS Customizations **/` ... `/** END DCS Customizations **/`
- **INI:** `;;; DCS Customizations` ... `;;; END DCS Customizations`

### Single-line annotations (no END needed)

- **Go:** `// DCS Customizations`

### Important

- Spelling is `Customizations` (plural, with a z). Fix any typos encountered.
- Every DCS file that is entirely new (not modifying upstream) does NOT need markers.
- The `_dcs.go` suffix convention identifies DCS companion files.

---

## Naming Conventions

| Type | Convention | Example |
| ---- | ---------- | ------- |
| Go companion files | `{original}_dcs.go` | `models/repo/repo_dcs.go` |
| DCS-only packages | Dedicated directory | `modules/dcs/`, `models/door43metadata/` |
| DCS route registration | `routes.go` in dcs package | `routers/api/v1/dcs/routes.go` |
| DCS templates (partials) | `templates/dcs/*.tmpl` | `templates/dcs/repo_header_badges.tmpl` |
| DCS templates (pages) | `templates/catalog/*.tmpl` | `templates/catalog/catalog.tmpl` |
| DCS JS features | `dcs-*.ts` | `web_src/js/features/dcs-info-icons.ts` |
| DCS CSS | `dcs.css` | `web_src/css/dcs.css` |
| DCS assets | `public/assets/img/dcs/` | `public/assets/img/dcs/uw.png` |

---

## Branch Strategy and Workflow

### How upstream Gitea develops

Gitea works on `main` until they are ready for an alpha/beta release, then creates a `release/v#.##` branch. After that, selected fixes from `main` are backported to the release branch via separate PRs.

### DCS branching model

DCS avoids maintaining separate backports by using a **shared base branch** that merges into both targets:

```text
upstream/main ─────────────────────────────────────────► (evolving)
                  \
                   fork point (merge-base)
                  /                        \
upstream/release/v1.25 ─────────────────────► (evolving)
                  |
            dcs-base-v1.25    ◄── All DCS customizations go here FIRST
                  |       \
                  ▼        ▼
      release/dcs/v1.25    main-and-base ──► main
      (production)         (integration)
```

### Branch purposes

| Branch | Purpose | Base |
| ------ | ------- | ---- |
| `dcs-base-v#.##` | Single source of truth for ALL DCS changes | `git merge-base upstream/main upstream/release/v#.##` |
| `release/dcs/v#.##` | Production release; upstream release + DCS | `upstream/release/v#.##` + merge `dcs-base-v#.##` |
| `main-and-base` | Integration branch for merging DCS into main | `main` + merge `dcs-base-v#.##` |
| `main` | Tracks upstream main + DCS customizations | Target of `main-and-base` PRs |

### Day-to-day development workflow

1. **All new DCS work goes to `dcs-base-v#.##` first** — features, bug fixes, refactoring
2. **Merge `dcs-base-v#.##` → `release/dcs/v#.##`** via PR to get production releases
3. **Merge `dcs-base-v#.##` → `main-and-base`** to keep main in sync
4. Fix any API differences in the target branches (not in `dcs-base-v#.##`)

### Why this works

- `dcs-base-v#.##` branches off the **merge-base** of `upstream/main` and `upstream/release/v#.##`, so its code is compatible with both (at the time of the fork point)
- As upstream evolves, both targets may introduce API changes; these are handled during the merge into each target, not in the base branch
- Git preserves the merge history, so repeated merges of `dcs-base-v#.##` into targets work correctly — only new commits are merged each time

### Handling API differences between targets

When upstream `main` has newer APIs than `release/v#.##` (common as main evolves faster):

- The `dcs-base-v#.##` code uses the **older API** (from the merge-base point)
- When merging into `release/dcs/v#.##`: usually no conflicts (release has the same or similar API)
- When merging into `main-and-base`: conflicts arise where APIs changed; resolve by adapting DCS code to the newer `main` API
- These adaptations live only in `main-and-base`, not in `dcs-base-v#.##`

### Pulling in upstream updates

To incorporate upstream changes into the release branch:

```bash
git fetch upstream
git checkout release/dcs/v#.##
git merge upstream/release/v#.##
# Resolve any conflicts with DCS code, run pre-PR checklist
```

To incorporate upstream changes into main:

```bash
git fetch upstream
git checkout main-and-base
git merge origin/main    # or: git merge upstream/main
# Resolve conflicts, run pre-PR checklist
```

### Creating a new dcs-base for a new Gitea version

When upstream creates `release/v#.##` (e.g., moving from v1.25 to v1.26):

```bash
# 1. Find the merge-base (the commit where main and the new release diverge)
git fetch upstream
git merge-base upstream/main upstream/release/v#.## # → <base-commit>

# 2. Create the new DCS base branch from that commit
git checkout -b dcs-base-v#.## <base-commit>

# 3. Cherry-pick or merge all DCS commits from the previous base branch
#    Claude Code can help port these, adapting for any API changes
git cherry-pick <first-dcs-commit>..<last-dcs-commit>
#    OR: git merge dcs-base-v<prev> (then fix conflicts)

# 4. Create the new release branch
git checkout -b release/dcs/v#.## upstream/release/v#.##
git merge dcs-base-v#.##

# 5. Create integration branch for main
git checkout -b main-and-base-v#.## origin/main
git merge dcs-base-v#.##
```

**Tip:** Use `git log --oneline dcs-base-v<prev>` to see all DCS commits that need porting. Claude Code can automate the cherry-pick and conflict resolution process.

---

## Merge Conflict Resolution

### Using pattern rules

1. For each conflicted file, check `dcs-customizations.md` for the pattern type
2. Apply the merge rule for that pattern (see Modification Patterns above)
3. For STRUCT_FIELD: keep upstream changes, ensure DCS fields remain at end
4. For ROUTE_BLOCK: re-add RegisterDCS*() call in correct scope
5. For FUNC_CALL: verify function signature compatibility, re-add call
6. For LOGIC_MOD: manually integrate both sets of changes
7. For template INJECT: re-add `{{template "dcs/..."}}` at appropriate point

### After any merge

1. `go mod tidy && go mod vendor` - sync vendor directory
2. `make build` - must compile
3. Run the Pre-PR Checklist (lint, spell, tests)
4. Verify DCS markers: `grep -r "DCS Customiz" --include="*.go" --include="*.tmpl" | wc -l`
5. Test key DCS features: catalog, healthcheck, repo metadata display

### If function signatures changed upstream

1. Check `services/convert/repository_dcs.go` - ToRepoDCS() may need parameter updates
2. Check `models/repo/repo_dcs.go` - LoadLatestDMs() may need updates
3. Check `routers/api/v1/dcs/routes.go` - middleware function signatures may change
4. Check all FUNC_CALL sites in `dcs-customizations.md`

---

## Localization

### Locale file format

- `release/v1.25` and earlier: **INI format** (`options/locale/locale_en-US.ini`)
- `main` (upstream v1.26+): **JSON format** (`options/locale/locale_en-US.json`)

### DCS locale key conventions

- **New DCS keys** are prefixed with `dcs.` (e.g., `dcs.repo.metadata.title`) and placed in a clearly marked block at the end of the locale file. This makes them instantly identifiable and conflict-free during upstream merges.
- **Override keys** that change an upstream value (e.g., `repo.code` = `"Files"` instead of `"Code"`) keep their original key name and stay in their original position in the file so merge conflicts reveal upstream changes.
- **Cron task keys** use the upstream naming convention `admin.dashboard.<taskname>` because the key is constructed dynamically in `services/cron/tasks.go`. These are placed in a DCS block near other dashboard keys.

### INI file markers

```ini
;;; DCS Customizations
dashboard.update_metadata = Update Door43 Metadata
;;; END DCS Customizations
```

### JSON file markers

JSON does not support comments, so DCS keys are grouped at the end of the file with `dcs.` prefix for identification.

### Adding new DCS locale keys

1. Choose a key name with `dcs.` prefix (e.g., `dcs.repo.metadata.newfield`)
2. Add to the appropriate locale file (INI for release branch, JSON for main)
3. Reference in templates as `{{ctx.Locale.Tr "dcs.repo.metadata.newfield"}}`
4. Reference in Go as `ctx.Tr("dcs.repo.metadata.newfield")`

---

## Template Architecture

DCS template modifications use two approaches:

1. **DCS Partials** (`templates/dcs/*.tmpl`) - Extracted blocks included via `{{template "dcs/..." .}}`. Reduces inline modifications to a single line. Preferred approach.

2. **Full replacements** - Some templates are entirely DCS content (e.g., `templates/home.tmpl`). These are documented as FULL_REPLACE in `dcs-customizations.md`. Upstream changes to these files should be evaluated but likely don't apply.

3. **Gitea extension points** (`templates/custom/*.tmpl`) - Gitea's built-in extension system. Used for navbar links, extra tabs, footer scripts. These never conflict with upstream.

DCS template helper functions are registered in `modules/templates/helper.go` and implemented in `modules/dcs/`.

---

## Working with Claude Code on this fork

### Recommended workflow

1. **Describe the DCS change** you want to make; Claude Code will put changes in the right files using DCS naming conventions and comment markers
2. **Run the Pre-PR Checklist** — Claude Code can run all lint/build checks and fix issues automatically
3. **For merges**, describe which branches are involved and let Claude Code handle conflict resolution using the pattern rules in `dcs-customizations.md`
4. **For version transitions** (new `dcs-base-v#.##`), Claude Code can automate cherry-picking DCS commits and adapting to API changes

### What Claude Code should know

- Always check `dcs-customizations.md` before modifying any existing Gitea file — it documents the pattern type and dependencies
- New DCS-only files do NOT need comment markers; modified Gitea files MUST have them
- When fixing CI failures, check the GitHub Actions logs via `gh run view <id> --log-failed --repo unfoldingWord/dcs`
- The `dcs-base-v#.##` branch should never contain target-specific adaptations (API differences between `main` and `release`); those go in the target branches only
