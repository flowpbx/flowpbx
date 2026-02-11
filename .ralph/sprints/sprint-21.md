# Sprint 21 — Flutter Calling (Outbound & Inbound)

**Phase**: 1E (Mobile App)
**Focus**: Make and receive calls, in-call UI, media handling
**Dependencies**: Sprint 20

**PRD Reference**: Section 9.1 (Core Features), Section 9.2 (Platform Requirements)

## Tasks

### Outbound Calls
- [x] Create dialpad screen with number input and call button
- [x] Implement contact directory from PBX API (extension list)
- [x] Implement contact search / filter on dialpad
- [ ] Implement outbound SIP INVITE to PBX
- [ ] Implement codec negotiation: G.711 alaw/ulaw, Opus
- [ ] Implement SRTP for encrypted media over untrusted networks

### Inbound Calls
- [ ] Create full-screen incoming call UI (caller ID, accept/reject buttons)
- [ ] Handle incoming SIP INVITE → show incoming call screen
- [ ] Implement call accept (200 OK) and reject (486 Busy)
- [ ] Implement ringtone playback for incoming calls

### In-Call Screen
- [ ] Create in-call screen layout: caller info, duration timer, action buttons
- [ ] Implement mute/unmute toggle
- [ ] Implement speaker/earpiece toggle
- [ ] Implement hold/resume
- [ ] Implement DTMF pad (send RFC 2833 telephone-event)
- [ ] Implement blind transfer to extension or number
- [ ] Implement hangup (send BYE)
- [ ] Handle remote hangup (receive BYE) — return to idle

### Audio Session
- [ ] Configure iOS audio session for VoIP (AVAudioSession)
- [ ] Configure Android audio focus management
- [ ] Handle audio routing: earpiece, speaker, Bluetooth, wired headset
- [ ] Implement proximity sensor control (screen off when near ear)
