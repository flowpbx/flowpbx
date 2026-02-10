# Sprint 02 — Database Layer

**Phase**: 1A (Foundation)
**Focus**: SQLite integration, migration system, core schema, repository pattern
**Dependencies**: Sprint 01

**PRD Reference**: Section 4 (Data Model), Section 3 (Technology Stack)

## Tasks

- [x] Add SQLite dependency (`modernc.org/sqlite` or `mattn/go-sqlite3`) — benchmark and choose
- [x] Create `internal/database/database.go` — open SQLite with WAL mode, connection setup
- [x] Create embedded migration system (embed SQL files via `//go:embed`, run on startup)
- [x] Migration 001: `system_config` table (key-value config store)
- [x] Migration 002: `extensions` table (per PRD Section 4.4)
- [x] Migration 003: `trunks` table (per PRD Section 4.2)
- [x] Migration 004: `inbound_numbers` table (per PRD Section 4.3)
- [x] Migration 005: `voicemail_boxes` table (per PRD Section 4.5)
- [x] Migration 006: `voicemail_messages` table (per PRD Section 4.6)
- [x] Migration 007: `ring_groups` table (per PRD Section 4.7)
- [x] Migration 008: `ivr_menus` table (per PRD Section 4.8)
- [x] Migration 009: `time_switches` table (per PRD Section 4.9)
- [x] Migration 010: `call_flows` table (per PRD Section 4.10)
- [x] Migration 011: `cdrs` table (per PRD Section 4.11)
- [x] Migration 012: `registrations` table (per PRD Section 4.12)
- [x] Migration 013: `conference_bridges` table (per PRD Section 4.13)
- [x] Create repository interfaces for all entities (CRUD operations)
- [x] Implement encrypted field support for passwords (AES-256-GCM, key from config)
- [x] Implement system_config repository (get/set key-value, load all into memory on startup)
