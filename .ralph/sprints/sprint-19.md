# Sprint 19 — Push Gateway & Flutter App

**Phase**: 1E (Mobile App & Push Gateway)
**Focus**: Push gateway service, PBX integration, Flutter softphone app
**Dependencies**: Sprint 08, Sprint 14

**PRD Reference**: Section 9 (Flutter Softphone App), Section 10 (Push Gateway & License Server)

## Tasks

### Push Gateway (`cmd/pushgw` + `internal/pushgw/`)
- [ ] Create push gateway handlers in `internal/pushgw/` (shares Go module with PBX)
- [ ] Create PostgreSQL schema: licenses, installations, push_logs
- [ ] Implement FCM integration (Firebase Admin SDK for Go)
- [ ] Implement APNs integration (HTTP/2 provider API)
- [ ] Implement `POST /v1/push` — validate license → send push → log result
- [ ] Implement `POST /v1/license/validate` — validate license key, return entitlements
- [ ] Implement `POST /v1/license/activate` — activate new installation (generate instance_id)
- [ ] Implement `GET /v1/license/status` — check license status
- [ ] Implement rate limiting per license key
- [ ] Containerize push gateway for deployment

### PBX ↔ Push Gateway Integration
- [ ] Create `internal/push/client.go` — push gateway HTTP client
- [ ] On incoming call to offline extension: check registrations for push token → send push request
- [ ] Implement push wait: hold call for configurable timeout (default 5s) waiting for app to register
- [ ] If no registration within timeout: continue flow (voicemail, next node, etc.)
- [ ] Implement push token management: store/update/invalidate via registration

### Flutter Softphone App
- [ ] Create Flutter project, configure state management (Riverpod or Bloc)
- [ ] Create login screen: server URL + extension number + password
- [ ] Evaluate and integrate SIP library (dart_sip_ua / native bridge)
- [ ] Implement SIP registration over TLS/TCP
- [ ] Implement outbound calls: dialpad, contact search
- [ ] Implement inbound calls: full-screen incoming call UI
- [ ] Implement iOS CallKit integration (native call UI, lock screen answering)
- [ ] Implement Android ConnectionService integration
- [ ] Create in-call screen: mute, speaker, hold, DTMF pad, transfer, hangup
- [ ] Implement call history (from PBX API `/api/v1/app/history`, cached locally)
- [ ] Implement voicemail list + playback (from `/api/v1/app/voicemail`, stream audio)
- [ ] Implement DND toggle (update PBX via `/api/v1/app/me`)
- [ ] Implement follow-me toggle
- [ ] Set up FCM (Android) and APNs/PushKit (iOS) push notification handling
- [ ] Implement push wake-up: on push received → wake SIP stack → register → receive INVITE
- [ ] Implement SRTP support for encrypted media
- [ ] Implement codec support: G.711, Opus
- [ ] Handle background audio sessions (iOS/Android)
- [ ] Create app authentication: `POST /api/v1/app/auth` → JWT + SIP config
- [ ] Implement `POST /api/v1/app/push-token` — register push token with PBX

### App API Endpoints (PBX side)
- [ ] Implement `POST /api/v1/app/auth` — extension login, return JWT + SIP config
- [ ] Implement `GET /api/v1/app/me` — extension profile
- [ ] Implement `PUT /api/v1/app/me` — update DND, follow-me
- [ ] Implement `GET /api/v1/app/voicemail` — list voicemails for boxes linked to this extension
- [ ] Implement `PUT /api/v1/app/voicemail/:id/read` — mark read
- [ ] Implement `GET /api/v1/app/voicemail/:id/audio` — stream audio
- [ ] Implement `GET /api/v1/app/history` — call history for this extension
- [ ] Implement `POST /api/v1/app/push-token` — register FCM/APNs token
- [ ] Implement JWT auth middleware for app endpoints
