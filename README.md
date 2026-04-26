# Coppermind

**Your personal Plex, but for books.**

Coppermind is a self-hosted digital library for ebooks and audiobooks. Run it on your home server, NAS, or a cheap VPS. Your family browses, reads, and listens from any device.

## Features

- 📚 **Ebook & Audiobook support** — EPUB, MOBI, MP3, M4B, PDF
- 🔍 **Search & Browse** — by title, author, series, and tags
- 📖 **In-browser EPUB reader** — server-side rendered, chapter navigation
- 🎧 **Audiobook player** — HTML5 audio with track list
- 👥 **Multi-user** — admin and viewer roles with per-user reading progress
- 📊 **Reading progress** — track where you left off across devices
- 📱 **Mobile-first UI** — responsive design, dark mode with system preference
- 📡 **OPDS catalog** — compatible with KOReader, Moon+ Reader, Thorium, etc.
- 📧 **Send to Kindle** — email books directly to your Kindle
- 🏷️ **Shelves & Tags** — organize your collection
- ⭐ **Ratings & Reviews** — per-user ratings
- 🐳 **Single binary** — one executable, one SQLite database, one media directory

## Quick Start

### Binary

```bash
# Initialize database and create admin user
coppermind init --name "My Library" --username admin --password secret

# Import books
coppermind import /path/to/book.epub
coppermind bulk-import /path/to/books/

# Start server
coppermind serve --host 0.0.0.0 --port 5000
```

### Docker

```bash
docker compose up -d
```

Then open http://localhost:5000 and complete the setup wizard.

### Docker Compose

```yaml
services:
  coppermind:
    image: coppermind:latest
    ports:
      - "5000:5000"
    volumes:
      - ./data:/data
    environment:
      - COPPERMIND_HOST=0.0.0.0
    restart: unless-stopped
```

## Configuration

Coppermind follows the [12-factor app](https://12factor.net/) methodology. Configure via environment variables (preferred) or a JSON config file.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `COPPERMIND_DB` | `./data/coppermind.db` | SQLite database path |
| `COPPERMIND_DATA_DIR` | `./data` | Library files directory |
| `COPPERMIND_HOST` | `127.0.0.1` | Bind address |
| `COPPERMIND_PORT` | `5000` | Bind port |
| `COPPERMIND_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `COPPERMIND_SESSION_SECRET` | auto-generated | Session signing secret (hex) |
| `COPPERMIND_ALLOW_GUESTS` | `false` | Allow unauthenticated browsing |
| `COPPERMIND_SMTP_HOST` | | SMTP server for Send-to-Kindle |
| `COPPERMIND_SMTP_PORT` | `587` | SMTP port |
| `COPPERMIND_SMTP_USER` | | SMTP username |
| `COPPERMIND_SMTP_PASS` | | SMTP password |
| `COPPERMIND_SMTP_FROM` | | From email address |

### Config File

Optionally, create `config.json` in the working directory or `~/.config/coppermind/config.json`:

```json
{
  "host": "0.0.0.0",
  "port": 5000,
  "data_dir": "/data",
  "allow_guests": true
}
```

Environment variables always override the config file.

## CLI Commands

```
coppermind init              Initialize database and first library
coppermind serve             Start the web server
coppermind import <path>     Import a file or directory
coppermind bulk-import <dir> Import all supported files from a directory
coppermind add-user          Add a new user
coppermind reset-password    Reset a user's password
coppermind list-users        List all users
coppermind list              List works, authors, or series
coppermind search            Search for works
```

## API

### JSON API

All endpoints under `/api/v1/`:

- `GET /api/v1/works` — list works (paginated, filterable)
- `GET /api/v1/works/:id` — work detail with editions
- `GET /api/v1/editions/:id` — edition detail
- `GET /api/v1/authors` — list authors
- `GET /api/v1/series` — list series
- `GET /api/v1/me/reading` — current reading states (authenticated)

### OPDS Catalog

OPDS feeds at `/api/v1/opds/` for e-reader apps:

- Root catalog, new acquisitions, by author, by series, search

## Architecture

- **Go** single binary with embedded templates and assets
- **SQLite** database with WAL mode
- **HTMX** for interactive UI without a JS framework
- **Chi** router with middleware stack
- **bcrypt** password hashing, HMAC session cookies
- Proper **Work/Edition** data model (a work can have EPUB + audiobook editions)

## Development

```bash
# Build
make build

# Run tests
make test

# Build and run
make dev

# Cross-compile releases
make release

# Build Docker image
make docker
```

## License

MIT
