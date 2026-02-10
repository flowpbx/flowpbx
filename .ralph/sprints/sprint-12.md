# Sprint 12 — Flow Node Implementations

**Phase**: 1C (Flow Engine & Nodes)
**Focus**: Implement all flow node handler functions
**Dependencies**: Sprint 11

**PRD Reference**: Section 5 (Flow Graph Node Types), Section 5.3 (Flow Engine)

## Tasks

- [x] Implement Inbound Number node handler (entry point, DID matching — mostly a passthrough to first edge)
- [x] Implement Extension node handler (ring extension with timeout, outputs: answered / no-answer)
- [x] Implement Ring Group node handler — ring_all strategy
- [x] Implement Ring Group node handler — round_robin strategy
- [x] Implement Ring Group node handler — random strategy
- [ ] Implement Ring Group node handler — longest_idle strategy
- [ ] Implement Time Switch node handler (evaluate rules against current time + timezone, follow matching edge or default)
- [ ] Implement IVR Menu node handler (play prompt, collect DTMF digits, route by digit match, handle timeout + invalid)
- [ ] Implement Voicemail node handler (play greeting from target box, record to WAV, store message, trigger MWI)
- [ ] Implement Play Message node handler (play audio file via RTP, then continue to next edge)
- [ ] Implement Hangup node handler (terminate call with configurable cause code)
- [ ] Implement Set Caller ID node handler (override caller ID name/number for downstream nodes)
- [ ] Implement Transfer node handler (blind transfer to external number or extension)
- [ ] Implement Conference node handler (join caller into conference bridge)
- [ ] Create stub for Webhook node handler (future — log and continue)
- [ ] Create stub for Queue node handler (future — route to extension, log warning)
