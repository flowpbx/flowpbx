# Sprint 22 — Flutter Platform Integration & Push Notifications

**Phase**: 1E (Mobile App)
**Focus**: CallKit, ConnectionService, push notification wake-up
**Dependencies**: Sprint 21

**PRD Reference**: Section 9.1 (Core Features), Section 9.2 (Platform Requirements), Section 10.3 (Push Flow)

## Tasks

### iOS — CallKit
- [x] Integrate CallKit for native iOS call UI
- [x] Implement CXProvider for incoming calls (lock screen answering)
- [x] Implement CXCallController for outgoing calls
- [x] Handle CallKit actions: answer, end, mute, hold, DTMF
- [x] Implement call directory integration (caller ID lookup)

### Android — ConnectionService
- [x] Implement ConnectionService for native Android call integration
- [x] Handle incoming call notification (heads-up notification with answer/reject)
- [x] Handle outgoing call routing through ConnectionService
- [x] Implement foreground service for active calls

### Push Notifications
- [ ] Set up Firebase Cloud Messaging (FCM) for Android
- [ ] Set up APNs / PushKit for iOS (VoIP push for call wake-up)
- [ ] Register push token with PBX via `POST /api/v1/app/push-token`
- [ ] Re-register token on app update or token refresh
- [ ] Implement push wake-up flow: receive push → wake SIP stack → register → receive INVITE
- [ ] Handle push payload: extract caller_id, call_id for pre-display
- [ ] Implement push timeout handling: if SIP registration fails within 5s, show missed call
- [ ] Test push delivery when app is backgrounded, killed, and device locked

### Background & Battery
- [ ] Implement background audio session handling (iOS/Android)
- [ ] Handle app backgrounding: maintain SIP registration or rely on push
- [ ] Battery optimization whitelisting guidance in-app (Android)
- [ ] Implement WiFi lock for active calls (Android)
