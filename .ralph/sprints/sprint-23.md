# Sprint 23 — Flutter App Features

**Phase**: 1E (Mobile App)
**Focus**: Call history, voicemail, DND, follow-me, contacts, polish
**Dependencies**: Sprint 21

**PRD Reference**: Section 9.1 (Core Features), Section 7 (App API Endpoints)

## Tasks

### Call History
- [x] Create call history screen (list view: caller/callee, direction, duration, timestamp)
- [ ] Fetch history from PBX API `GET /api/v1/app/history`
- [ ] Implement local caching of call history (SQLite or Hive)
- [ ] Implement pull-to-refresh and pagination
- [ ] Tap to call back from history entry
- [ ] Show missed call badge count

### Voicemail
- [ ] Create voicemail list screen (shows messages from all boxes linked to extension)
- [ ] Fetch voicemails from `GET /api/v1/app/voicemail`
- [ ] Implement audio playback: stream from `GET /api/v1/app/voicemail/:id/audio`
- [ ] Implement playback controls: play/pause, seek, speed (1x/1.5x/2x)
- [ ] Mark as read on playback via `PUT /api/v1/app/voicemail/:id/read`
- [ ] Show unread voicemail badge count
- [ ] Pull-to-refresh voicemail list

### Settings & Toggles
- [ ] Create settings screen
- [ ] Implement DND toggle (update PBX via `PUT /api/v1/app/me`)
- [ ] Implement follow-me toggle (update PBX via `PUT /api/v1/app/me`)
- [ ] Show current registration status and server info
- [ ] Implement re-login / change server
- [ ] Show app version and about info

### Contacts
- [ ] Create contacts screen: list of PBX extensions with name, number, online status
- [ ] Fetch from PBX extensions API
- [ ] Implement search/filter
- [ ] Tap to call from contact
- [ ] Show online/offline presence indicators (registration status)

### Polish
- [ ] Implement app-wide error handling and user-friendly error messages
- [ ] Add loading states and skeleton screens
- [ ] Implement network connectivity monitoring (show offline banner)
- [ ] Test on iOS 15+ and Android 10+
- [ ] Test on WiFi, 4G, 5G network conditions
