# sqlc Evaluation

Recommendation: do not introduce `sqlc` right now.

The store layer is now easier to maintain with explicit SQL, shared helpers, focused interfaces, batched list enrichment, embedded migrations, and `sqlx` row structs for the most repetitive user/edition scans. The remaining boilerplate is manageable and still easy to read.

## Queries that could benefit

`sqlc` would help most with simple, stable CRUD/list queries where the SQL shape is fixed:

- `GetLibrary` / `ListLibraries`
- `GetAuthor` / `FindAuthorBySortName` / `ListAuthors`
- `GetSeries` / `FindSeriesByName` / `ListSeries`
- `GetTrack` / `ListTracks`
- reading-state and rating lookups

These would gain generated row structs and compile-time query/result checking.

## Queries that would likely stay handwritten

Several important store operations are dynamic or orchestration-heavy, which reduces the value of generated queries:

- `ListWorks`, because filtering, sorting, paging, and enrichment are dynamic.
- `UpdateWork`, `UpdateEdition`, and `UpdateUser`, because they are partial updates.
- `MergeWorks`, because it is a transaction with several dependent statements.
- Import/matching flows, because they compose multiple store operations.

## Tradeoff

Adding `sqlc` would introduce a code generation step and another layer of generated types that still need mapping into domain types. Given the current size of the store package, `sqlx` plus explicit row structs provides most of the practical benefit without extra build tooling.

Revisit `sqlc` if:

- the schema grows substantially,
- more fixed-shape reporting queries are added,
- manual scan code starts dominating store changes again, or
- compile-time SQL/result checking becomes more valuable than avoiding codegen.
