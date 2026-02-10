# Sprint 03 — HTTP Server & Admin Auth

**Phase**: 1A (Foundation)
**Focus**: chi router, middleware stack, session auth, first-boot wizard API
**Dependencies**: Sprint 02

**PRD Reference**: Section 6.2 (Admin Auth), Section 6.3 (Embedded SPA), Section 7 (REST API), Section 11 (Configuration & First Boot)

## Tasks

- [x] Create `internal/api/server.go` — chi router setup, mount all route groups
- [ ] Create auth middleware (session-based, secure cookie + CSRF token)
- [ ] Create logging middleware (request ID, method, path, status, duration)
- [ ] Create recovery middleware (panic recovery, log stack trace)
- [ ] Create CORS middleware (configurable origins)
- [ ] Implement Argon2id password hashing utility
- [ ] Create admin user table (migration) — username, password_hash, totp_secret (nullable, for Phase 2)
- [ ] Implement `POST /api/v1/auth/login` — validate credentials, create session
- [ ] Implement `POST /api/v1/auth/logout` — destroy session
- [ ] Implement `GET /api/v1/auth/me` — return current admin user
- [ ] Implement `GET /api/v1/health` — unauthenticated health check
- [ ] Create first-boot detection (empty admin_users table)
- [ ] Implement setup wizard API: `POST /api/v1/setup` — create admin account, set hostname, configure SIP ports
- [ ] Set up static file serving via `//go:embed` with SPA fallback (non-API routes → `index.html`)
- [ ] Implement consistent JSON response envelope `{ "data": ..., "error": ... }`
- [ ] Add pagination helpers (`?limit=N&offset=N`) for list endpoints
