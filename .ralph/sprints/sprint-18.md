# Sprint 18 — Call Recording & Follow-Me

**Phase**: 1D (Conference, Recording & Follow-Me)
**Focus**: Call recording, follow-me with external numbers, recording management
**Dependencies**: Sprint 07, Sprint 08

**PRD Reference**: Section 8.7 (Follow-Me), Section 4.4 (Extensions — recording_mode, follow_me)

## Tasks

- [x] Create `internal/media/recorder.go` — fork RTP stream to WAV writer (separate goroutine, non-blocking)
- [x] Implement per-extension recording config: always / off / on_demand
- [x] Implement per-trunk recording config
- [x] Implement global recording policy setting
- [x] Organize recording files by date: `$DATA_DIR/recordings/YYYY/MM/DD/call_{id}.wav`
- [x] Store recording_file path in CDR on recording completion
- [x] Implement recording retention policy: auto-delete recordings older than configurable days
- [x] Add storage usage monitoring (total recordings size)
- [x] Create recording browser API with playback stream and download
- [x] Implement follow-me: sequential ring — ring registered devices → after timeout → ring external numbers via trunk
- [x] Implement follow-me: simultaneous ring option (ring all at once)
- [x] Implement external number dialling via outbound trunk for follow-me legs
- [x] Implement confirmation prompt on external legs ("Press 1 to accept this call") to prevent voicemail pickup
- [ ] Implement per-destination ring timeout in follow-me config
- [ ] Add follow-me config UI per extension (enable/disable, add external numbers with delay/timeout)
- [ ] Create follow-me toggle in app API: `PUT /api/v1/app/me` (follow_me_enabled)
