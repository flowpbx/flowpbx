# Sprint 08 — Internal Calls (Extension to Extension)

**Phase**: 1B (Call Handling)
**Focus**: INVITE handling, call setup, multi-device ringing, call bridging, BYE/CANCEL
**Dependencies**: Sprint 05, Sprint 07

**PRD Reference**: Section 8.3 (Inbound Call Handling), Section 8.5 (Media Proxy)

## Tasks

- [x] Create `internal/sip/invite.go` — INVITE handler: identify call type (internal vs inbound vs outbound)
- [x] Implement internal call routing: look up target extension → find all active registrations
- [x] Send `100 Trying` immediately on receiving INVITE
- [x] Implement multi-device ringing: fork INVITE to all registered contacts for target extension
- [x] Implement `180 Ringing` and `183 Session Progress` relay (early media)
- [x] Implement call answer: `200 OK` from first answering device → ACK → cancel other forks
- [ ] Create `internal/sip/dialog.go` — dialog/session state management (track active calls)
- [ ] Implement media bridging: allocate RTP proxy session, rewrite SDP for both legs
- [ ] Implement BYE handling — tear down both legs, release media, update CDR
- [ ] Implement CANCEL handling — caller hangs up before answer, cancel all forks
- [ ] Implement busy detection (486 Busy Here) — all devices busy or DND enabled
- [ ] Implement ring timeout — if no answer within extension's `ring_timeout`, return no-answer
- [ ] Create active calls tracking (in-memory map of call ID → call state)
