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
- [x] Implement outbound SIP INVITE to PBX
- [x] Implement codec negotiation: G.711 alaw/ulaw, Opus
- [x] Implement SRTP for encrypted media over untrusted networks

### Inbound Calls
- [x] Create full-screen incoming call UI (caller ID, accept/reject buttons)
- [x] Handle incoming SIP INVITE → show incoming call screen
- [x] Implement call accept (200 OK) and reject (486 Busy)
- [x] Implement ringtone playback for incoming calls

### In-Call Screen
- [x] Create in-call screen layout: caller info, duration timer, action buttons
- [x] Implement mute/unmute toggle
- [x] Implement speaker/earpiece toggle
- [x] Implement hold/resume
- [x] Implement DTMF pad (send RFC 2833 telephone-event)
- [x] Implement blind transfer to extension or number
- [x] Implement hangup (send BYE)
- [x] Handle remote hangup (receive BYE) — return to idle

### Audio Session
- [x] Configure iOS audio session for VoIP (AVAudioSession)
- [x] Configure Android audio focus management
- [x] Handle audio routing: earpiece, speaker, Bluetooth, wired headset
- [x] Implement proximity sensor control (screen off when near ear)
