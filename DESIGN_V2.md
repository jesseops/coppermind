# Coppermind v2 — Greenfield Design Document

## What This Document Is

A comprehensive design for rebuilding Coppermind as a shareable, multi-user
digital library application. This is not a refactoring guide for the existing
codebase — it's a from-scratch architecture that takes everything learned from
v1 and designs it correctly from day one.

The existing codebase (~9,000 lines of Go) proves the concept works. This
document identifies what it got right, what it got wrong, and how to build
something suitable for sharing publicly and running for multiple users.

---

## 1. Product Vision

Coppermind is a self-hosted digital library for ebooks and audiobooks. You run
it on your home server, NAS, or a cheap VPS. Your family browses, reads, and
listens from any device. You manage the collection. Power users can send books
directly to their e-readers.

**One-line pitch:** "Your personal Plex, but for books."

### Core Use Cases

1. **The Librarian** (admin) imports books, fixes metadata, organizes series,
   removes duplicates, and maintains the collection.
2. **The Reader** (family member) browses the library on their phone or tablet,
   reads an EPUB in the browser, or downloads it to their Kindle/Kobo.
3. **The Listener** picks up an audiobook where they left off, across devices.
4. **The Curator** tags books, builds shelves/collections ("Dad's Sci-Fi Picks"),
   and rates what they've read.

### Design Principles

- **Local-first.** No cloud dependencies. Works on a Raspberry Pi.
- **Single binary.** One executable, one SQLite database, one media directory.
- **Progressive complexity.** Single user works with zero config. Multi-user,
  e-reader sync, and collections are opt-in features, not mandatory concepts.
- **Mobile-first UI.** Phones are the primary browsing device for family use.
- **Respect the domain.** Books have works, editions, authors, series, and
  publishers. The data model should reflect how publishing actually works,
  not just how files are stored.

---

## 2. Lessons from v1

### What v1 Got Right
- **Single binary with embedded assets** — go:embed for templates/static is
  excellent for deployment. Keep this.
- **SQLite** — perfect for the use case. No external database needed.
- **HTMX** — server-rendered HTML with partial swaps is the right choice for
  this kind of app. No JS framework needed.
- **Automatic metadata extraction** — EPUB OPF parsing, MOBI EXTH headers,
  cover extraction. This is table-stakes functionality.
- **CLI + web** — the CLI is genuinely useful for bulk operations, scripting,
  and initial setup. Keep both interfaces.

### What v1 Got Wrong

**Data model is too flat.** The `items` table mixes the concept of "a book"
(intellectual work) with "a file I have" (a specific EPUB/MOBI/M4B file).
The same novel might exist as an EPUB, a MOBI, a hardcover scan, and an
audiobook — these are different *editions* of the same *work*. v1 hacks
around this with display-side grouping by `title + author`, but the database
doesn't know they're related. This causes:
- The "merge" feature exists because the DB can't represent versions properly.
- Series ordering is per-item, not per-work.
- No way to say "I have the EPUB and audiobook of Dune" without it looking
  like two separate books in some views.

**Auth is bolted on.** v1 went through three auth iterations (Basic Auth →
admin password → session cookies) because auth wasn't designed upfront. The
admin/viewer split works for one family, but can't scale to "multiple users
with their own reading progress."

**No user identity.** Reading position, audiobook progress, shelves, and
ratings all need to be per-user. v1's playback_positions table has a single
row per item — there's no concept of "whose position is this?"

**Tailwind browser build.** Loading the full Tailwind CSS compiler in the
browser on every page load is heavy and slow on mobile. The production app
should use a precompiled CSS file.

**No OPDS or e-reader delivery.** The most common ask for book servers is
"send it to my Kindle." v1 has no support for this.

---

## 3. Data Model

The core insight: separate **what a book is** (the work) from **what files
you have** (editions/files) and **what users think about it** (progress,
ratings, shelves).

### Entity Relationship

```
User ──┐
       │ many-to-many
       ├──── UserShelf (shelves/collections per user)
       ├──── ReadingState (progress per user per edition)
       │
Work ──┤
       │ one-to-many
       ├──── Edition (EPUB, MOBI, audiobook, etc.)
       │       │
       │       ├──── EditionFile (the actual file on disk)
       │       └──── Track (audiobook chapters)
       │
       ├──── WorkAuthor (many-to-many with Author, includes role)
       │
       └──── belongs to Series (with position)

Author ── standalone entity, normalized

Series ── standalone entity with name and optional description

Library ── a container for works (for multi-library support)
```

### Tables

```sql
-- ── Identity ────────────────────────────────────────────────────────

CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL UNIQUE,
    display_name  TEXT    NOT NULL,
    password_hash TEXT    NOT NULL,       -- bcrypt
    role          TEXT    NOT NULL DEFAULT 'viewer',  -- 'admin' | 'viewer'
    created_at    TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Bibliographic ───────────────────────────────────────────────────

CREATE TABLE authors (
    id            INTEGER PRIMARY KEY,
    name          TEXT    NOT NULL,          -- display name: "Brandon Sanderson"
    sort_name     TEXT    NOT NULL,          -- "sanderson, brandon"
    created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE series (
    id            INTEGER PRIMARY KEY,
    name          TEXT    NOT NULL,
    description   TEXT,
    created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE works (
    id              INTEGER PRIMARY KEY,
    library_id      INTEGER NOT NULL REFERENCES libraries(id),
    title           TEXT    NOT NULL,
    sort_title      TEXT    NOT NULL,        -- "way of kings, the"
    description     TEXT,
    series_id       INTEGER REFERENCES series(id),
    series_index    REAL,                    -- supports "2.5" for novellas
    language        TEXT,                    -- ISO 639-1: "en", "es"
    first_published INTEGER,                -- year
    cover_path      TEXT,                    -- path to best available cover
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE work_authors (
    work_id   INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    author_id INTEGER NOT NULL REFERENCES authors(id),
    role      TEXT    NOT NULL DEFAULT 'author',  -- 'author' | 'narrator' | 'editor' | 'translator'
    PRIMARY KEY (work_id, author_id, role)
);

CREATE TABLE work_tags (
    work_id INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    tag     TEXT    NOT NULL,
    PRIMARY KEY (work_id, tag)
);

-- ── Editions & Files ────────────────────────────────────────────────

CREATE TABLE editions (
    id              INTEGER PRIMARY KEY,
    work_id         INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    edition_type    TEXT    NOT NULL,        -- 'ebook' | 'audiobook'
    format          TEXT,                    -- 'epub' | 'mobi' | 'mp3' | 'm4b' | 'pdf'
    isbn            TEXT,
    publisher       TEXT,
    published_year  INTEGER,
    narrator        TEXT,                    -- for audiobooks
    duration_seconds INTEGER,               -- for audiobooks
    file_path       TEXT,                    -- path to the file on disk
    file_hash       TEXT,                    -- SHA-256 for dedup
    file_size       INTEGER,                -- bytes
    cover_path      TEXT,                    -- edition-specific cover
    notes           TEXT,
    status          TEXT    NOT NULL DEFAULT 'active',
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE tracks (
    id                INTEGER PRIMARY KEY,
    edition_id        INTEGER NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    track_index       INTEGER NOT NULL,
    title             TEXT,
    duration_seconds  INTEGER,
    file_path         TEXT    NOT NULL,
    file_hash         TEXT,
    created_at        TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── User State ──────────────────────────────────────────────────────

CREATE TABLE reading_states (
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    edition_id      INTEGER NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    status          TEXT    NOT NULL DEFAULT 'unread',  -- 'unread' | 'reading' | 'finished' | 'abandoned'
    progress        REAL    NOT NULL DEFAULT 0,         -- 0.0–1.0 (percentage)
    -- For ebooks:
    chapter_index   INTEGER,
    scroll_position REAL,
    -- For audiobooks:
    track_index     INTEGER,
    position_seconds REAL,
    -- Timestamps:
    started_at      TEXT,
    finished_at     TEXT,
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, edition_id)
);

CREATE TABLE user_ratings (
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    work_id   INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    rating    INTEGER,           -- 1–5 stars, NULL = unrated
    review    TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, work_id)
);

CREATE TABLE shelves (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    description TEXT,
    is_public   INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE shelf_works (
    shelf_id  INTEGER NOT NULL REFERENCES shelves(id) ON DELETE CASCADE,
    work_id   INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    added_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (shelf_id, work_id)
);

-- ── Libraries ───────────────────────────────────────────────────────

CREATE TABLE libraries (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ── Schema versioning ───────────────────────────────────────────────

CREATE TABLE schema_version (
    version INTEGER NOT NULL
);
```

### Key Design Decisions

**Works vs. Editions.** A "work" is "Dune by Frank Herbert." An "edition" is
"the 2019 Ace EPUB" or "the Audible audiobook narrated by Simon Vance." This
lets the UI show "Dune" once with badges for [EPUB] [AUDIOBOOK] instead of
two separate entries. Series position, tags, and ratings attach to the work.
Format-specific metadata (ISBN, narrator, file path) attaches to the edition.

**Authors are normalized.** Instead of a free-text `author` column, authors
are a separate table with a many-to-many join. This eliminates the
"Sanderson, Brandon" vs. "Brandon Sanderson" problem at the data layer.
The `work_authors` table includes a `role` column so the same person can be
author on one work and narrator on another.

**Reading state is per-user, per-edition.** A user might be reading the EPUB
and their spouse listening to the audiobook — these are separate progress
records. The `reading_states` table unifies ebook and audiobook progress in
one row.

**Shelves replace ad-hoc notes.** Instead of a free-text notes field for
organization, users create named shelves ("Currently Reading", "Sci-Fi
Favorites", "Book Club 2025"). Shelves are per-user but can be marked public
for sharing.

**Tags are shared, ratings are personal.** Tags on works are global (anyone
can see them, admin manages them). Ratings and reviews are per-user.

---

## 4. Authentication & Authorization

### User Model

| Role | Can browse | Can read/listen | Can download | Can edit metadata | Can import | Can manage users |
|------|-----------|----------------|-------------|-------------------|-----------|-----------------|
| `viewer` | ✓ | ✓ | ✓ | ✗ | ✗ | ✗ |
| `admin` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

### Authentication Flow

- **Login page** at `/login`. Form-based (not Basic Auth).
- Passwords stored as bcrypt hashes. No plaintext anywhere.
- Session cookie: `coppermind_session`, HttpOnly, Secure (when HTTPS),
  SameSite=Strict. Contains a signed JWT or HMAC token with user ID and
  expiry.
- Session duration: 30 days (remember me) or 24 hours (default).
- CSRF: double-submit cookie pattern (same as v1, proven to work).

### First-Run Setup

On first launch with an empty database:
1. Redirect to `/setup`.
2. Create the admin account (username + password).
3. Create the first library.
4. Redirect to the import page.

No config file needed for basic setup. `admin_pass` config is gone — replaced
by real user accounts.

### Guest Access (Optional)

For families who don't want per-user accounts:
- Config option: `allow_guests: true`
- Guests can browse, read, listen, download — but have no reading history,
  shelves, or ratings.
- Admin still requires login.

---

## 5. API Design

### Principles
- Server-rendered HTML with HTMX for the primary UI.
- JSON API at `/api/v1/` for programmatic access (OPDS, e-reader send, etc.).
- All state-changing operations go through the same service layer regardless
  of whether the caller is HTML or JSON.

### Key Endpoints (HTML)

```
# ── Public ──────────────────────────────────────────
GET  /                         Library home (recently added)
GET  /works/:id                Work detail (all editions, reviews)
GET  /authors                  Author browse
GET  /authors/:id              Author detail
GET  /series                   Series browse
GET  /series/:id               Series detail (ordered by series_index)
GET  /shelves                  Public shelves
GET  /shelves/:id              Shelf contents

GET  /read/:edition_id         EPUB reader
GET  /listen/:edition_id       Audiobook player
GET  /preview/:edition_id      Text preview
GET  /download/:edition_id     Download file

# ── Auth ────────────────────────────────────────────
GET  /login                    Login form
POST /login                    Authenticate
GET  /logout                   Clear session
GET  /setup                    First-run setup

# ── User ────────────────────────────────────────────
GET  /me                       Current user profile
GET  /me/reading               Currently reading
GET  /me/shelves               My shelves
POST /me/shelves               Create shelf
POST /me/rate/:work_id         Rate a work

# ── Admin ───────────────────────────────────────────
GET  /admin/import             Import page
POST /admin/import             Upload files
GET  /admin/works/:id/edit     Edit work metadata
POST /admin/works/:id          Save edits
POST /admin/works/:id/delete   Delete work
GET  /admin/users              User management
GET  /admin/duplicates         Duplicate detection
POST /admin/send/:edition_id   Send to e-reader
```

### Key Endpoints (JSON API)

```
GET  /api/v1/works             List works (paginated, filterable)
GET  /api/v1/works/:id         Work detail
GET  /api/v1/editions/:id      Edition detail
GET  /api/v1/me/reading        Current reading states
POST /api/v1/me/reading/:id    Update reading progress
GET  /api/v1/opds              OPDS catalog root
GET  /api/v1/opds/new          OPDS new acquisitions
GET  /api/v1/opds/authors      OPDS author list
```

---

## 6. E-Reader Integration

### OPDS Catalog

OPDS (Open Publication Distribution System) is the standard protocol for
e-reader apps to browse and download from a library. Implementing OPDS means
any OPDS-compatible app (KOReader, Moon+ Reader, Aldiko, Thorium) can browse
and download books.

**Feeds to implement:**
- Root catalog (`/api/v1/opds`)
- New acquisitions (sorted by date added)
- Authors (navigation feed → acquisition feed per author)
- Series (navigation feed → acquisition feed per series)
- Search (OpenSearch descriptor)

Each entry includes download links for available formats, cover images,
and metadata (author, description, series, ISBN).

### Send to Kindle / E-Reader

**Kindle (via email):**
- User configures their Send-to-Kindle email address in their profile.
- Admin configures SMTP settings in the server config.
- "Send to Kindle" button on edition cards.
- Server emails the EPUB/MOBI file to the Kindle address.
- Kindle automatically converts and delivers.

**Kobo / Generic (via USB or download):**
- Download button already exists.
- Consider: Kobo sync protocol support is complex and probably not worth it.
  Download + sideload is fine.

### Format Conversion

The most common need: convert EPUB → MOBI for older Kindles (pre-2022) or
EPUB → KEPUB for Kobo optimization.

Options:
- **Shell out to Calibre's `ebook-convert`** — if installed. This is how
  most book servers handle it.
- **Don't convert in v2.** Modern Kindles accept EPUB directly. KEPUB is
  a nice-to-have, not essential. Document "install Calibre CLI if you want
  conversion" and add it later.

Recommendation: skip conversion in v1 of v2. Support it as a plugin/hook
later.

---

## 7. Import & Metadata

### Import Pipeline

```
File(s) uploaded or path provided
    │
    ├── Detect format (EPUB, MOBI, MP3, M4B, PDF)
    │
    ├── Extract metadata:
    │     EPUB → OPF (title, author, series, series_index, ISBN,
    │                  description, language, publisher, cover)
    │     MOBI → EXTH headers (same fields, less reliable)
    │     MP3  → ID3 tags (title, artist, album)
    │     M4B  → MP4 metadata (title, artist, album)
    │     PDF  → XMP metadata (limited; title, author)
    │
    ├── Match to existing Work:
    │     Look up by title + author (normalized).
    │     If match found → create new Edition under existing Work.
    │     If no match → create new Work + Edition.
    │
    ├── Match to existing Author:
    │     Normalize name. Look up by sort_name.
    │     If match → link. If not → create.
    │
    ├── Match to existing Series:
    │     Normalize name. Look up by name.
    │     If match → link. If not → create.
    │
    ├── Copy file to library directory.
    ├── Extract cover art → save as cover file.
    ├── Compute SHA-256 hash for dedup.
    │
    └── Duplicate detection:
          If file_hash matches an existing edition → flag as duplicate,
          don't import (or offer to skip/replace).
```

### Metadata Sources (Future)

For v2 launch, metadata comes only from the file itself. Future enhancement:
- **Open Library API** — look up by ISBN or title+author.
- **Google Books API** — cover images, descriptions.
- **MusicBrainz** — audiobook metadata when ID3/MP4 tags are sparse.

These are fetched on demand, not automatically, to respect API rate limits
and user privacy.

---

## 8. Technology Choices

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Language | Go | Single binary, fast, good stdlib. Proven in v1. |
| Database | SQLite (via modernc.org/sqlite) | No external deps. WAL mode for concurrency. Proven in v1. |
| SQL layer | sqlx | Lightweight, no ORM magic. Proven in v1. |
| HTTP router | chi | Clean middleware, route groups. Proven in v1. |
| Templates | html/template | Standard lib, secure by default. Proven in v1. |
| Interactivity | HTMX | Server-rendered partials, no JS build step. Proven in v1. |
| CSS | Tailwind CLI (precompiled) | v1 uses the browser build which is slow. Precompile at build time. |
| Auth | bcrypt + HMAC session cookies | Simple, secure, no external auth service. |
| E-reader | OPDS + SMTP (Kindle) | Industry standards. |
| CLI | Cobra | Proven in v1. |
| Container | Docker (alpine) | Proven in v1. |
| CI | GitHub Actions | Proven in v1. |

### What Changes from v1

| v1 | v2 | Why |
|----|----|-----|
| Tailwind browser build | Tailwind CLI precompiled | 400KB JS on every page load → zero |
| Alpine.js | Removed (already done in late v1) | Caused HTMX swap bugs. Vanilla JS is fine. |
| Flat `items` table | Works + Editions + Authors | Proper domain modeling |
| `dbPath` passed everywhere | Repository pattern with DI | Testable, clean interfaces |
| Single admin password | User accounts with bcrypt | Multi-user support |
| No API | JSON API + OPDS | E-reader integration, automation |
| `go:embed` for all assets | `go:embed` for templates, Tailwind precompiled | Smaller, faster |

---

## 9. Directory Structure

```
coppermind/
├── cmd/
│   └── coppermind/
│       ├── main.go
│       ├── root.go          # CLI setup, config loading
│       ├── serve.go         # `run` command
│       ├── import.go        # `import` / `bulk-import` commands
│       ├── users.go         # `add-user` / `reset-password` commands
│       └── ...
├── internal/
│   ├── domain/              # Core types (Work, Edition, Author, User, etc.)
│   │   ├── work.go
│   │   ├── edition.go
│   │   ├── author.go
│   │   ├── user.go
│   │   └── ...
│   ├── store/               # Database access (repository pattern)
│   │   ├── store.go         # Store interface
│   │   ├── sqlite.go        # SQLite implementation
│   │   ├── works.go         # Work queries
│   │   ├── editions.go      # Edition queries
│   │   ├── users.go         # User queries
│   │   ├── migrate.go       # Schema migrations
│   │   └── ...
│   ├── importer/            # Import pipeline
│   │   ├── importer.go      # Orchestrator
│   │   ├── epub.go          # EPUB metadata extraction
│   │   ├── mobi.go          # MOBI metadata extraction
│   │   ├── audio.go         # MP3/M4B metadata extraction
│   │   └── matcher.go       # Work/author/series matching
│   ├── auth/                # Authentication & sessions
│   │   ├── auth.go
│   │   ├── session.go
│   │   └── password.go
│   ├── opds/                # OPDS feed generation
│   │   └── opds.go
│   ├── sender/              # E-reader delivery
│   │   ├── kindle.go        # SMTP send-to-kindle
│   │   └── sender.go        # Interface
│   └── web/                 # HTTP layer
│       ├── server.go        # Server setup, middleware
│       ├── routes.go        # Route registration
│       ├── handlers/        # One file per handler group
│       │   ├── library.go   # Browse, search, filter
│       │   ├── reader.go    # EPUB reader
│       │   ├── player.go    # Audiobook player
│       │   ├── admin.go     # Import, edit, users
│       │   ├── auth.go      # Login, logout, setup
│       │   ├── api.go       # JSON API
│       │   └── opds.go      # OPDS feeds
│       ├── views/           # View models (not domain types)
│       │   └── ...
│       ├── templates/       # HTML templates
│       │   └── ...
│       └── static/          # CSS, icons
│           └── ...
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── tailwind.config.js
└── go.mod
```

---

## 10. What to Build First

### Phase 1: Foundation (weeks 1–2)
- Schema, migrations, store layer with tests.
- Domain types.
- Import pipeline (EPUB, MOBI, MP3/M4B).
- CLI: init, import, bulk-import, list, search.

### Phase 2: Web UI — Browse (weeks 3–4)
- Server setup, middleware, auth (users, login, sessions).
- Library browse (recently added, search, filter).
- Work detail page.
- Author and series browse.
- Cover art serving.
- First-run setup flow.

### Phase 3: Reading & Listening (weeks 5–6)
- EPUB reader with chapter nav.
- Audiobook player with track list.
- Reading state persistence (per user, per edition).
- "Currently Reading" view.

### Phase 4: E-Reader & Sharing (weeks 7–8)
- OPDS catalog.
- Send-to-Kindle (SMTP).
- Download with format badges.
- Shelves and ratings.

### Phase 5: Polish (weeks 9–10)
- Mobile optimization.
- Precompiled Tailwind CSS.
- Docker image.
- README and deployment docs.

---

## 12. What Not to Build

These are explicitly out of scope for the foreseeable future. Documenting
them prevents scope creep.

- **Social features** (activity feeds, friend libraries).
- **DRM management** or store integrations.
- **Format conversion** (defer to Calibre CLI).
- **Cloud sync** between multiple Coppermind instances.
- **Full-text search** inside book content (search metadata only).
- **PDF reader** (PDFs can be downloaded; in-browser PDF rendering is its own
  project).
- **Native mobile apps** (the web UI is the mobile experience).
- **Multi-language UI** (English only for now; use standard i18n patterns so
  it's possible later).
