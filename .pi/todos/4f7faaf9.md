{
  "id": "4f7faaf9",
  "title": "11. Port MOBI metadata & cover extraction",
  "tags": [
    "phase-1",
    "import"
  ],
  "status": "done",
  "created_at": "2026-04-26T14:16:20.702Z"
}

## Goal
Port v1's MOBI metadata extraction into `internal/importer/mobi.go`.

## What to port from v1
- `extractMobiMetadata` → parse EXTH headers for title (503), author (100), ISBN
- `extractMobiCover` → find cover record via EXTH 201/202, extract image bytes
- `mobiRecord0`, `parseEXTH`, `parseEXTHRaw` — PDB/MOBI binary parsing
- `mobiRecordOffset`, `mobiCoverRecordIndex`
- `decompressPalmDoc` — PalmDoc LZ77 decompression for text preview
- `extractMobiTextPreview`
- `detectImageExt` — magic byte detection for PNG/JPG/GIF

## Changes from v1
- Return `metadata.Extracted` struct (same as EPUB task)
- Better error handling with wrapped errors
