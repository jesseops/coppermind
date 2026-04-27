# AGENTS.md — Context for AI Agents

## What This Project Is

Coppermind is a self-hosted ebook/audiobook library server ("Plex for books").
It's a greenfield Go rewrite of an earlier ~9k-line codebase at `~/code/coppermind-codex`.
The design spec is in `DESIGN_V2.md` — read it for product vision and data model rationale.

## Key Architectural Decisions

**Templates use per-page cloning.** All HTML templates in `internal/web/templates/` are parsed
individually against a cloned base template. This is required because Go's `html/template`
uses a flat namespace for `{{define}}` blocks — parsing all pages into one template set causes
the last-parsed `"content"` block to win. See commit `1f26547` for the fix. When adding a new
page template, it's automatically picked up (no registration needed).

**SQLite single-connection constraint.** `MaxOpenConns(1)` is set to avoid SQLite locking.
This means you **cannot hold open `*sql.Rows` while making sub-queries** on the same store.
Always collect rows into a slice, close the rows, then do follow-up queries. `ListWorks` was
deadlocking before this was fixed (commit `9a40cbb`).

**Work/Edition split.** The core data model separates "what a book is" (Work) from "what files
you have" (Edition). A Work can have multiple Editions (EPUB + audiobook). Series position,
tags, and ratings attach to the Work. Format-specific data attaches to the Edition. This is
the main improvement over v1's flat `items` table.

**CSRF exemptions.** The `/receive/*` API endpoints (generate, status, download), `/api/v1/*`,
and `/api/v1/opds/*` are outside the CSRF middleware group because e-reader browsers and API
clients don't handle CSRF cookies. The `POST /receive/send/{edition_id}` endpoint (called from
the main UI) IS CSRF-protected. See `buildRouter()` in `server.go`.

**Config precedence.** Environment variables always override the JSON config file (12-factor).
Config is loaded once at startup via `internal/config/`. The session secret is auto-generated
if not set, which means sessions are invalidated on restart unless `COPPERMIND_SESSION_SECRET`
is persisted.

## Dependencies Worth Knowing

- `pgaskin/kepubify/v4/kepub` — pure Go EPUB→KEPUB converter for Kobo devices. Used in the
  send-to-ereader feature. No external binary needed.
- `modernc.org/sqlite` — pure Go SQLite (no CGO). Slower than `mattn/go-sqlite3` but enables
  `CGO_ENABLED=0` builds and simpler cross-compilation.
- HTMX is vendored at `internal/web/static/js/htmx.min.js` (v2.0.4).
- **Tailwind v4** standalone CLI (`bin/tailwindcss`, gitignored). Run `make css-install` to
  download it. Source CSS is `internal/web/static/css/input.css`; the built `app.css` is
  committed so that `go:embed` works without a Tailwind build step in CI/Docker.

## Theming

Themes are pure CSS via `[data-theme="<name>"]` on `<html>`. All components reference
CSS custom properties — there are zero theme-specific component selectors. Everything
from colors to fonts to shadows is driven by `--theme-*` variables.

Three choices: `auto` (archives for light system pref, vault for dark), `archives`
(parchment/serif light), `vault` (dark/copper/monospace). Theme selection is stored
in `localStorage('coppermind-theme')` and applied before first paint.

To add a theme: define a new `[data-theme="<name>"]` block in `input.css` with all
`--theme-*` variables, add a button to the picker in `base.html`, and run `make css`.

## Things That Don't Exist Yet (Intentionally)

- **Format conversion** — No EPUB→MOBI (KindleGen is discontinued). Modern Kindles read EPUB.
- **External metadata lookup** — Open Library, Google Books APIs are future work.
- **Full-text search** — Only metadata search is implemented.
- **PDF reader** — PDFs can be downloaded but not read in-browser.
- **Migration from v1** — No data migration tool from the old `items` table schema.

## Testing

Run `make test`. Store tests use in-memory SQLite (`:memory:`). Importer tests generate
synthetic EPUBs programmatically (see `createTestEpub` in `epub_test.go`). There are no
real EPUB/MOBI fixtures checked in.

## File Layout Conventions

- `internal/domain/` — Pure value types, no DB imports. Helper methods like `GenerateSortTitle`.
- `internal/store/` — All DB access behind the `Store` interface. `SQLiteStore` is the only impl.
- `internal/importer/` — Format detection, metadata extraction, matching, file operations.
- `internal/web/` — HTTP server, handlers, templates, static assets. Handlers are in
  `handlers.go`, `api.go`, `opds_handlers.go`, `user_handlers.go`, `send.go`.
- `internal/auth/` — Password hashing, session cookies, middleware, CSRF.
- `cmd/coppermind/` — CLI entry point. One file per command group.
