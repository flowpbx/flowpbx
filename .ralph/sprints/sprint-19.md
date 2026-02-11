# Sprint 19 — Push Gateway & App API

**Phase**: 1E (Mobile App & Push Gateway)
**Focus**: Push gateway service, PBX push integration, app-facing API endpoints
**Dependencies**: Sprint 08, Sprint 14

**PRD Reference**: Section 10 (Push Gateway & License Server), Section 7 (App API Endpoints)

## Tasks

### Push Gateway (`cmd/pushgw` + `internal/pushgw/`)
- [x] Create push gateway handlers in `internal/pushgw/` (shares Go module with PBX)
- [x] Create PostgreSQL schema: licenses, installations, push_logs
- [x] Implement FCM integration (Firebase Admin SDK for Go)
- [x] Implement APNs integration (HTTP/2 provider API)
- [x] Implement `POST /v1/push` — validate license → send push → log result
- [x] Implement `POST /v1/license/validate` — validate license key, return entitlements
- [x] Implement `POST /v1/license/activate` — activate new installation (generate instance_id)
- [x] Implement `GET /v1/license/status` — check license status
- [x] Implement rate limiting per license key
- [x] Containerize push gateway for deployment

### PBX ↔ Push Gateway Integration
- [x] Create `internal/push/client.go` — push gateway HTTP client
- [x] On incoming call to offline extension: check registrations for push token → send push request
- [x] Implement push wait: hold call for configurable timeout (default 5s) waiting for app to register
- [x] If no registration within timeout: continue flow (voicemail, next node, etc.)
- [ ] Implement push token management: store/update/invalidate via registration

### App API Endpoints (PBX side)
- [ ] Implement JWT auth middleware for app endpoints
- [ ] Implement `POST /api/v1/app/auth` — extension login, return JWT + SIP config
- [ ] Implement `GET /api/v1/app/me` — extension profile
- [ ] Implement `PUT /api/v1/app/me` — update DND, follow-me
- [ ] Implement `GET /api/v1/app/voicemail` — list voicemails for boxes linked to this extension
- [ ] Implement `PUT /api/v1/app/voicemail/:id/read` — mark read
- [ ] Implement `GET /api/v1/app/voicemail/:id/audio` — stream audio
- [ ] Implement `GET /api/v1/app/history` — call history for this extension
- [ ] Implement `POST /api/v1/app/push-token` — register FCM/APNs token
