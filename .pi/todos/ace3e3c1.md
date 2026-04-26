{
  "id": "ace3e3c1",
  "title": "13. Implement work/author/series matcher for import",
  "tags": [
    "phase-1",
    "import"
  ],
  "status": "open",
  "created_at": "2026-04-26T14:16:43.435Z",
  "assigned_to_session": "019dca13-bab2-757b-8af3-670d85b8173b"
}

## Goal
Implement `internal/importer/matcher.go` — the logic that takes extracted metadata and matches/creates Works, Authors, and Series in the store.

## Matching rules (from DESIGN_V2.md §7)

### Author matching
1. Normalize name → sort_name ("Brandon Sanderson" → "sanderson, brandon")
2. Look up by sort_name in authors table
3. If found → use existing author. If not → create new author.
4. Port v1's `normalizeAuthorForKey`, `canonicalizeAuthorKey`, `splitAuthorNames` logic

### Series matching
1. Normalize series name (trim, lowercase for comparison)
2. Look up by name (case-insensitive) in series table
3. If found → use existing. If not → create new.

### Work matching
1. Normalize title + primary author name
2. Look up existing work with same normalized title + author in same library
3. If found → create new Edition under existing Work
4. If not found → create new Work + Edition

### Dedup
- Compute SHA-256 of the file
- Check editions.file_hash for match
- If exact hash match → flag as duplicate, don't import (return error/warning)

## Interface
```go
type MatchResult struct {
    Work       *domain.Work     // existing or newly created
    Authors    []domain.Author  // existing or newly created
    Series     *domain.Series   // existing or newly created, may be nil
    IsNewWork  bool
    IsDuplicate bool
    DuplicateEditionID int64
}

func (m *Matcher) Match(ctx context.Context, meta metadata.Extracted, libraryID int64) (*MatchResult, error)
```
