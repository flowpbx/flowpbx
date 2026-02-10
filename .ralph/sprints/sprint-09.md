# Sprint 09 — Inbound & Outbound Calls via Trunk

**Phase**: 1B (Call Handling)
**Focus**: DID matching for inbound, trunk selection for outbound, caller ID, prefix manipulation
**Dependencies**: Sprint 06, Sprint 08

**PRD Reference**: Section 8.3 (Inbound Call Handling), Section 8.4 (Outbound Call Handling), Section 4.3 (Inbound Numbers)

## Tasks

- [x] Implement inbound call matching: INVITE To/Request-URI → match against `inbound_numbers` table
- [x] Route matched inbound call to destination extension (direct routing, before flow engine)
- [x] Pass through caller ID from trunk on inbound calls
- [x] Implement outbound dialling: extension sends INVITE to external number
- [x] Implement outbound trunk selection: ordered by priority field, skip failed/disabled trunks
- [x] Implement prefix manipulation: strip N leading digits (`prefix_strip`), add prefix (`prefix_add`)
- [x] Implement caller ID rules for outbound: use extension CID, trunk CID, or override
- [x] Implement max_channels enforcement per trunk (reject if at limit)
- [x] Create inbound numbers CRUD API: `GET/POST/PUT/DELETE /api/v1/numbers`
- [x] Create extensions CRUD API: `GET/POST/PUT/DELETE /api/v1/extensions`
- [ ] Implement `GET /api/v1/extensions/:id/registrations` — list active registrations for extension
- [ ] Add inbound numbers CRUD page to admin UI
