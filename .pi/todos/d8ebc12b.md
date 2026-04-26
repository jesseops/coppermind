{
  "id": "d8ebc12b",
  "title": "23. Implement EPUB reader, audiobook player, reading state, download",
  "tags": [
    "phase-3",
    "web"
  ],
  "status": "done",
  "created_at": "2026-04-26T14:18:43.627Z"
}

## Goal
Implement the server-side EPUB reader and audiobook player with per-user reading state persistence.

## EPUB reader (`GET /read/:edition_id`)
- Handler: `internal/web/handlers/reader.go`
- Port v1's server-side approach: extract chapter HTML, serve in an iframe or styled container
- Chapter navigation (prev/next + chapter list sidebar)
- Chapter content: sanitized HTML from EPUB with CSS from EPUB
- Serve EPUB assets (images, CSS) via `/epub-asset/:edition_id/*path`
- Save reading position on chapter change (HTMX POST to `/api/v1/me/reading/:id`)
- Restore position on load
- Clean, distraction-free reading UI (hide nav, fullscreen-ish)

## Audiobook player (`GET /listen/:edition_id`)
- Handler: `internal/web/handlers/player.go`
- Track list with duration
- HTML5 audio player
- Serve audio files via `/audio/:track_id`
- Save position every ~10 seconds (debounced HTMX/fetch POST)
- Restore position on load (track + position_seconds)
- Continue across tracks

## Reading state API
- `POST /api/v1/me/reading/:edition_id` — save progress (JSON body)
- `GET /api/v1/me/reading/:edition_id` — get progress
- For ebooks: chapter_index, scroll_position, progress percentage
- For audiobooks: track_index, position_seconds, progress percentage

## "Currently Reading" (`GET /me/reading`)
- List editions the user has in-progress (status = 'reading')
- Show cover, title, progress bar, "Continue" link

## Download (`GET /download/:edition_id`)
- Serve the raw file with Content-Disposition: attachment
- Set proper Content-Type

## Templates
- `templates/reader.html` — EPUB reader
- `templates/player.html` — audiobook player
- `templates/reading.html` — currently reading list
