{
  "id": "add5ad4f",
  "title": "Add small builders for dynamic store SQL",
  "tags": [
    "db",
    "refactor"
  ],
  "status": "closed",
  "created_at": "2026-04-27T22:04:19.742Z"
}

Added lightweight dynamic SQL helpers:

- `whereBuilder` for clauses and args.
- `updateBuilder` for partial updates with consistent `updated_at` behavior.
- Applied builders to `UpdateUser`, `UpdateWork`, `UpdateEdition`, and `ListWorks`.

Validation: `go test ./...` passes.

Commit: `2f6fad7 refactor(store): add dynamic SQL builders`
