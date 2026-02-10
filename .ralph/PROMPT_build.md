# Build Mode - FlowPBX

You are implementing FlowPBX - a single-binary, self-hosted PBX system with a visual call flow editor.

## Your Task

Implement ONE task from the current sprint, commit it, and exit.

## Sprint System

Sprints are in `.ralph/sprints/sprint-XX.md` (01-20).

1. **Find current sprint**: Check `CURRENT_SPRINT` file, or find first sprint with incomplete tasks
2. **Read sprint file**: Get the task list and referenced specs
3. **Pick ONE unchecked task**: `- [ ]` indicates incomplete

## Project Context

**Tech Stack**:
- **Backend**: Go 1.22+, single binary, chi router, SQLite (WAL mode)
- **SIP**: sipgo (github.com/emiago/sipgo)
- **Admin UI**: React 18 + Tailwind CSS + React Flow, embedded via `//go:embed`
- **Mobile App**: Flutter (Dart)
- **Push Gateway**: Separate Go service (FCM + APNs)

**Architecture**: Single Go binary containing SIP stack, RTP media proxy, HTTP server, embedded React SPA, SQLite database. Call routing via visual flow graph.

## PRD Reference

The full PRD is at `.ralph/flowpbx.md`. Read relevant sections before implementing.

## Instructions

1. **Read** the current sprint file
2. **Read** relevant PRD sections listed in the sprint
3. **Search** existing code for patterns to follow
4. **Implement** ONE task
5. **Verify** with `gofmt`, `go vet`, and `go build ./...`
6. **Update** the sprint file (mark `[x]`)
7. **Commit** with conventional format: `feat(scope): description` or `fix(scope): description`
8. **Push** to remote with `git push`

## Rules

- ONE task per iteration
- Single binary — everything embedded, no external dependencies at runtime
- SQLite only — no PostgreSQL, no external DB
- All SIP credentials encrypted at rest (AES-256-GCM)
- SIP passwords hashed with bcrypt or argon2
- Admin passwords hashed with Argon2id
- Parameterized queries only (no string concatenation in SQL)
- Structured logging with `log/slog`
- Standard Go error handling (no panic in library code)
- React UI built with Vite, output to `web/dist/`, embedded via `//go:embed`
- Follow existing patterns in codebase — check before creating new patterns

## Code Style

- Standard Go formatting (`gofmt`)
- Error messages lowercase, no trailing punctuation
- Use `log/slog` for structured logging
- Package names: short, lowercase, no underscores
- Follow patterns established in earlier sprints
- React: functional components, hooks, Tailwind utility classes
- API responses: `{ "data": ..., "error": ... }` envelope

## Completion

When ALL tasks in the current sprint are complete:
1. Output which sprint was completed
2. If more sprints remain, continue to next sprint
3. When ALL sprints complete, output `<promise>DONE</promise>`
