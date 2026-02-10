# Sprint 17 — Conference Bridge

**Phase**: 1D (Conference, Recording & Follow-Me)
**Focus**: N-way audio mixing, conference room management, PIN entry
**Dependencies**: Sprint 07, Sprint 12

**PRD Reference**: Section 4.13 (Conference Bridges), Section 8.5 (Media Proxy — conference mixing)

## Tasks

- [x] Create `internal/media/mixer.go` — N-way audio mixing engine in RTP proxy
- [x] Implement conference room management: create room, join participant, leave, kick
- [ ] Implement PIN-protected conference entry (play prompt, collect digits, validate)
- [ ] Implement mute/unmute per participant
- [ ] Implement mute_on_join option (join muted by default)
- [ ] Implement announce_joins option (play tone or announcement on join/leave)
- [ ] Enforce max_members limit per conference bridge
- [ ] Implement conference recording (mix all participant audio → single WAV output)
- [ ] Track active conference participants in memory
- [ ] Expose active participants via API: `GET /api/v1/conferences/:id/participants`
- [ ] Add conference management UI: view active participants, mute/kick controls
