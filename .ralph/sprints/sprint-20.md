# Sprint 20 — Flutter Project Setup & SIP Registration

**Phase**: 1E (Mobile App)
**Focus**: Flutter project scaffolding, SIP library evaluation, auth flow, SIP registration
**Dependencies**: Sprint 19

**PRD Reference**: Section 9 (Flutter Softphone App), Section 9.3 (SIP Library), Section 9.4 (Authentication)

## Tasks

### Project Setup (`mobile/`)
- [x] Create Flutter project in `mobile/` directory within monorepo
- [x] Configure state management (Riverpod or Bloc)
- [x] Set up project structure: screens, services, models, widgets, providers
- [x] Configure app icon and branding assets
- [x] Set up linting rules (flutter_lints)
- [x] Add `mobile/` build commands to root Makefile

### SIP Library Evaluation
- [x] Evaluate `dart_sip_ua` — TLS, SRTP, Opus support, push wake-up capability
- [ ] Evaluate `flutter_ooh_sip` / native bridge options
- [ ] Evaluate fallback: native iOS (Swift) + Android (Kotlin) SIP engines with Flutter UI via platform channels
- [ ] Select library and document decision in PROMPT_learnings.md

#### Evaluation: `dart_sip_ua` (sip_ua on pub.dev)

**Package**: [sip_ua](https://pub.dev/packages/sip_ua) v0.x — ported from JsSIP
**GitHub**: [flutter-webrtc/dart-sip-ua](https://github.com/flutter-webrtc/dart-sip-ua)
**Last updated**: December 2025
**License**: MIT

| Criterion | Support | Notes |
|---|---|---|
| **TLS (SIP signaling)** | ⚠️ Partial | No native SIP-over-TLS (port 5061). Supports WSS (WebSocket Secure) and TCP. FlowPBX would need to add a WSS listener or implement a custom TLS transport using Dart's `SecureSocket`. |
| **SRTP (media encryption)** | ✅ Via WebRTC | Uses DTLS-SRTP via flutter_webrtc. All media encrypted by default. However, this is WebRTC-style SRTP (DTLS key exchange), not traditional SDES SRTP. FlowPBX media proxy would need DTLS-SRTP support. |
| **Opus codec** | ✅ Via WebRTC | Opus is WebRTC's default audio codec. Available automatically through flutter_webrtc. No explicit codec selection API — relies on SDP negotiation. |
| **Push wake-up** | ⚠️ Not built-in | No built-in push notification support. Requires separate integration with FCM (Android) + PushKit (iOS) + flutter_callkit_incoming. Community has documented the pattern but it requires significant custom work. |
| **G.711 (PCMU/PCMA)** | ✅ Via WebRTC | Supported as WebRTC fallback codecs. |
| **DTMF** | ✅ | RFC 2833 and SIP INFO methods supported. |
| **Background calls** | ❌ Poor | SIP connection drops when app goes to background/terminated. WebRTC session lost. Requires push-to-wake architecture. |
| **CallKit / ConnectionService** | ❌ Not included | Must add separately via `callkeep` or `flutter_callkit_incoming` packages. |

**Verdict**: dart_sip_ua is WebRTC-centric (ported from JsSIP browser library). It assumes a WebSocket/WebRTC architecture, not traditional SIP-over-TLS/UDP with SDES-SRTP. For FlowPBX — which is a traditional SIP PBX with RTP media proxy — this is a significant architecture mismatch. FlowPBX would need to add WebSocket and WebRTC/DTLS-SRTP support to the Go SIP stack, or dart_sip_ua would need a custom TLS transport layer. **Not recommended as primary choice.**

### Authentication & Registration
- [ ] Create login screen: server URL + extension number + SIP password
- [ ] Implement app auth flow: call `POST /api/v1/app/auth` → store JWT + SIP config
- [ ] Implement secure token storage (flutter_secure_storage)
- [ ] Implement SIP registration to PBX over TLS/TCP using credentials from auth response
- [ ] Implement auto-reconnect on network change (WiFi ↔ cellular)
- [ ] Implement registration status indicator in UI
- [ ] Implement logout: de-register SIP, clear tokens, return to login
