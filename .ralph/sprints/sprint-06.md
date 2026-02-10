# Sprint 06 — Trunk Registration & Management

**Phase**: 1B (Call Handling)
**Focus**: Outbound trunk registration, trunk health, IP-auth trunks, admin API
**Dependencies**: Sprint 05

**PRD Reference**: Section 8.2 (Trunk Registration), Section 4.2 (Trunks), Section 7 (Trunks API)

## Tasks

- [x] Create `internal/sip/trunk.go` — trunk registration client for register-type trunks
- [x] Implement periodic re-registration with configurable expiry
- [x] Implement registration failure handling with exponential backoff retry
- [x] Implement trunk health check via OPTIONS ping
- [x] Track trunk status (registered / failed / disabled) in memory + expose via API
- [x] Implement IP-auth trunk support (ACL-based, match source IP/CIDR, no registration)
- [x] Create trunk CRUD API handlers: `GET/POST/PUT/DELETE /api/v1/trunks`
- [x] Implement `GET /api/v1/trunks/:id` — include current registration status
- [x] Implement `POST /api/v1/trunks/:id/test` — attempt registration or OPTIONS ping, return result
- [x] Add trunk status to admin UI trunk list (green/red indicator)
- [ ] Load all enabled trunks on startup, begin registration
- [ ] Handle trunk enable/disable — start/stop registration on config change
