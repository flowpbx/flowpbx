# Sprint 23 — Flutter App Features

**Phase**: 1E (Mobile App)
**Focus**: Call history, voicemail, DND, follow-me, contacts, polish
**Dependencies**: Sprint 21

**PRD Reference**: Section 9.1 (Core Features), Section 7 (App API Endpoints)

## Tasks

### Call History
- [x] Create call history screen (list view: caller/callee, direction, duration, timestamp)
- [x] Fetch history from PBX API `GET /api/v1/app/history`
- [x] Implement local caching of call history (SQLite or Hive)
- [x] Implement pull-to-refresh and pagination
- [x] Tap to call back from history entry
- [x] Show missed call badge count

### Voicemail
- [x] Create voicemail list screen (shows messages from all boxes linked to extension)
- [x] Fetch voicemails from `GET /api/v1/app/voicemail`
- [x] Implement audio playback: stream from `GET /api/v1/app/voicemail/:id/audio`
- [x] Implement playback controls: play/pause, seek, speed (1x/1.5x/2x)
- [x] Mark as read on playback via `PUT /api/v1/app/voicemail/:id/read`
- [x] Show unread voicemail badge count
- [x] Pull-to-refresh voicemail list

### Settings & Toggles
- [x] Create settings screen
- [x] Implement DND toggle (update PBX via `PUT /api/v1/app/me`)
- [x] Implement follow-me toggle (update PBX via `PUT /api/v1/app/me`)
- [x] Show current registration status and server info
- [x] Implement re-login / change server
- [x] Show app version and about info

### Contacts
- [x] Create contacts screen: list of PBX extensions with name, number, online status
- [x] Fetch from PBX extensions API
- [x] Implement search/filter
- [x] Tap to call from contact
- [x] Show online/offline presence indicators (registration status)

### Polish
- [x] Implement app-wide error handling and user-friendly error messages
- [ ] Add loading states and skeleton screens
- [ ] Implement network connectivity monitoring (show offline banner)
- [ ] Test on iOS 15+ and Android 10+
- [ ] Test on WiFi, 4G, 5G network conditions
