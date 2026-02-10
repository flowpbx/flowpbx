# Sprint 14 — Voicemail System

**Phase**: 1C (Flow Engine & Nodes)
**Focus**: Voicemail recording, storage, MWI, email notification, admin UI
**Dependencies**: Sprint 12, Sprint 13

**PRD Reference**: Section 8.6 (Voicemail), Section 4.5 (Voicemail Boxes), Section 4.6 (Voicemail Messages), Section 7 (Voicemail API)

## Tasks

- [x] Create `internal/voicemail/voicemail.go` — voicemail recording: capture incoming RTP → write to WAV file
- [x] Implement configurable max recording duration (per voicemail box setting)
- [x] Play custom greeting per voicemail box (uploaded WAV) or default greeting fallback
- [x] Store voicemail message metadata in `voicemail_messages` table (caller ID, timestamp, duration, file_path)
- [x] Organize voicemail files by box: `$DATA_DIR/voicemail/box_{id}/msg_{id}.wav`
- [x] Store voicemail box greetings at `$DATA_DIR/greetings/box_{id}.wav`
- [x] Implement MWI: send SIP NOTIFY to extension linked via `notify_extension_id` on new message
- [x] Implement email notification with WAV attachment via SMTP (per-box `email_notify` + `email_address` settings)
- [x] Implement auto-delete of messages older than `retention_days` (per-box setting, cleanup goroutine)
- [ ] Implement max_messages limit per box (reject recording if at limit)
- [x] Create voicemail box CRUD API: `GET/POST/PUT/DELETE /api/v1/voicemail-boxes`
- [ ] Create voicemail message API: `GET /api/v1/voicemail-boxes/:id/messages`, `DELETE .../messages/:msg_id`, `PUT .../messages/:msg_id/read`, `GET .../messages/:msg_id/audio`
- [ ] Create `POST /api/v1/voicemail-boxes/:id/greeting` — upload custom greeting
- [ ] Create voicemail browser in admin UI: per-box message list, play, download, delete, mark read
- [ ] Add SMTP configuration to Settings page
