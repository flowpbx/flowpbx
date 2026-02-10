# Learnings

Runtime discoveries and gotchas captured during build iterations.

## Project Setup
<!-- Package manager, build tools, etc. -->

## Code Patterns

### API pagination envelope
List endpoints consumed by the frontend `list()` helper (in `web/src/api/client.ts`) **must** return a `PaginatedResponse` struct, not a raw array. The response shape after envelope unwrapping must be:
```json
{"items": [...], "total": N, "limit": N, "offset": N}
```
Use `parsePagination(r)` from `response.go` to extract `limit`/`offset` query params, then wrap results with `PaginatedResponse{Items, Total, Limit, Offset}`.

If the store `List()` method returns all records (no DB-level pagination), slice in-memory:
```go
total := len(all)
start := pg.Offset
if start > total { start = total }
end := start + pg.Limit
if end > total { end = total }
writeJSON(w, http.StatusOK, PaginatedResponse{Items: all[start:end], Total: total, Limit: pg.Limit, Offset: pg.Offset})
```

Endpoints that are **not** consumed via the frontend `list()` helper (e.g. time switches, ring groups, IVR menus, prompts, flows, conferences) return raw arrays and that's correct — their frontend pages use `get()` directly.

## Common Pitfalls

### Raw array vs PaginatedResponse mismatch
If a frontend page uses `listFoo()` which calls the `list()` client helper, the Go handler **must** return `PaginatedResponse`. Returning a raw array causes the frontend to get `undefined` for `res.items`, which crashes `DataTable` with `Cannot read properties of undefined (reading 'length')`. Always check the frontend API module to see whether it uses `list()` (paginated) or `get()` (raw) before writing a new list handler.
