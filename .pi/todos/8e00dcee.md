{
  "id": "8e00dcee",
  "title": "7. Implement SQLite store: works, authors, series",
  "tags": [
    "phase-1",
    "foundation"
  ],
  "status": "open",
  "created_at": "2026-04-26T14:15:38.105Z"
}

## Goal
Implement SQLite store methods for works, authors, and series — the core bibliographic entities.

## Files
- `internal/store/works.go`
- `internal/store/authors.go`
- `internal/store/series.go`

## Works
- CreateWork: insert into works, return with generated ID
- GetWork: by ID, join to get author names and edition count
- UpdateWork: partial update (title, sort_title, description, series_id, series_index, language, first_published, cover_path)
- DeleteWork: soft-delete or cascade delete editions
- ListWorks: with WorkFilter (libraryID, query, type, seriesID, authorID, sort, pagination). Return works with primary author name and edition type badges
- CountWorks: total count for pagination with same filter

## Authors
- CreateAuthor: insert with name and sort_name
- GetAuthor: by ID
- FindAuthorBySortName: lookup by normalized sort_name for dedup during import
- ListAuthors: all authors with work counts for a given library (via work_authors + works.library_id)
- LinkWorkAuthor / UnlinkWorkAuthor: manage work_authors join table
- GetWorkAuthors: all authors for a work with their roles

## Series
- CreateSeries: insert
- GetSeries: by ID
- FindSeriesByName: lookup for dedup during import (case-insensitive)
- ListSeries: all series with work counts for a library
- GetSeriesWorks: all works in a series ordered by series_index

## sort_name generation
Port v1's `normalizeAuthorForKey` logic but produce "lastname, firstname" format.
