# Sprint 02 — Database Layer

**Phase**: 1A (Foundation)
**Focus**: SQLite integration, migration system, core schema, repository pattern
**Dependencies**: Sprint 01

**PRD Reference**: Section 4 (Data Model), Section 3 (Technology Stack)

## Tasks

- [ ] Add SQLite dependency (`modernc.org/sqlite` or `mattn/go-sqlite3`) — benchmark and choose
- [ ] Create `internal/database/database.go` — open SQLite with WAL mode, connection setup
- [ ] Create embedded migration system (embed SQL files via `//go:embed`, run on startup)
- [ ] Migration 001: `system_config` table (key-value config store)
- [ ] Migration 002: `extensions` table (per PRD Section 4.4)
- [ ] Migration 003: `trunks` table (per PRD Section 4.2)
- [ ] Migration 004: `inbound_numbers` table (per PRD Section 4.3)
- [ ] Migration 005: `voicemail_boxes` table (per PRD Section 4.5)
- [ ] Migration 006: `voicemail_messages` table (per PRD Section 4.6)
- [ ] Migration 007: `ring_groups` table (per PRD Section 4.7)
- [ ] Migration 008: `ivr_menus` table (per PRD Section 4.8)
- [ ] Migration 009: `time_switches` table (per PRD Section 4.9)
- [ ] Migration 010: `call_flows` table (per PRD Section 4.10)
- [ ] Migration 011: `cdrs` table (per PRD Section 4.11)
- [ ] Migration 012: `registrations` table (per PRD Section 4.12)
- [ ] Migration 013: `conference_bridges` table (per PRD Section 4.13)
- [ ] Create repository interfaces for all entities (CRUD operations)
- [ ] Implement encrypted field support for passwords (AES-256-GCM, key from config)
- [ ] Implement system_config repository (get/set key-value, load all into memory on startup)
