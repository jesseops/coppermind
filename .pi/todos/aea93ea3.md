{
  "id": "aea93ea3",
  "title": "10. Port EPUB metadata & cover extraction",
  "tags": [
    "phase-1",
    "import"
  ],
  "status": "open",
  "created_at": "2026-04-26T14:16:12.308Z"
}

## Goal
Port and adapt v1's EPUB metadata extraction into `internal/importer/epub.go`.

## What to port from v1 (`internal/core/import.go`)
- `extractEpubMetadata` → parse container.xml → find OPF → extract title, author, series, series_index, year, ISBN
- `extractEpubCover` → find cover image in manifest, extract bytes
- `ExtractEpubChapter` → for server-side reader
- `ExtractEpubChapterText` → plain text extraction
- `ExtractEpubAsset` → serve CSS/images from EPUB
- OPF XML structures (containerXML, opfPackage, opfMetadata, etc.)
- ISBN validation (ISBN-10, ISBN-13)

## Changes from v1
- Return a `metadata.Extracted` struct (not `ItemInput`) with: Title, Authors []string, Series, SeriesIndex, ISBN, PublishedYear, Language, Publisher, Description, CoverData, CoverExt, Format
- Support multiple authors from OPF `<dc:creator>` elements
- Extract `<dc:description>` for work description
- Extract `<dc:language>` for language field
- Extract `<dc:publisher>` for publisher field

## Also port
- `readZipFile`, `resolveEpubPath` helpers
- `stripHTMLTagsWithBreaks`, `normalizeWhitespacePreserveLines` for text preview
- `decompressPalmDoc` — actually this is MOBI, not EPUB
