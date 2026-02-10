# Sprint 13 — Audio Prompt & DTMF Systems

**Phase**: 1C (Flow Engine & Nodes)
**Focus**: Audio prompt playback via RTP, DTMF collection, prompt upload/management
**Dependencies**: Sprint 07, Sprint 12

**PRD Reference**: Section 8.5 (Media Proxy), Section 5 (IVR Menu node), Section 7 (Audio Prompts API)

## Tasks

- [x] Create `internal/media/player.go` — audio prompt playback: read WAV file → packetize G.711 → send via RTP
- [x] Embed default system prompts in binary (WAV, G.711 format) via `//go:embed`
- [x] Extract default prompts to filesystem on first boot (`$DATA_DIR/prompts/system/`)
- [x] Create `internal/media/dtmf.go` — DTMF digit collection from RFC 2833 events during playback
- [x] Implement inter-digit timeout handling for multi-digit input
- [x] Implement max digits and terminator digit (#) support
- [ ] Implement per-call DTMF buffer management
- [ ] Create audio prompts CRUD API: `POST /api/v1/prompts` (upload), `GET /api/v1/prompts`, `GET /api/v1/prompts/:id/audio`, `DELETE /api/v1/prompts/:id`
- [ ] Implement audio format validation on upload (WAV, G.711 alaw/ulaw)
- [ ] Store custom prompts in `$DATA_DIR/prompts/custom/`
- [ ] Create audio prompt library page in admin UI (upload, list, play, delete)
