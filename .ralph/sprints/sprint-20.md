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
- [x] Evaluate `flutter_ooh_sip` / native bridge options
- [x] Select library and document decision in PROMPT_learnings.md

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

#### Evaluation: `flutter_ooh_sip` / native bridge options

The PRD references `flutter_ooh_sip` and `ooh_ooh` as candidate SIP libraries. **Neither package exists** on pub.dev or in any public repository — these appear to be placeholder names from the PRD authoring phase. The actual Flutter SIP ecosystem for native bridge approaches consists of the following options:

##### 1. Siprix VoIP SDK (`siprix_voip_sdk`)

**Package**: [siprix_voip_sdk](https://pub.dev/packages/siprix_voip_sdk) v1.0.33
**GitHub**: [siprix/FlutterPluginFederated](https://github.com/siprix/FlutterPluginFederated)
**Last updated**: December 2025
**License**: MIT (code), **commercial license required** (60-sec call limit in trial)

| Criterion | Support | Notes |
|---|---|---|
| **TLS (SIP signaling)** | ✅ Native | Full SIP-over-TLS support via native C++ SIP engine. Direct compatibility with FlowPBX SIP stack. |
| **SRTP (media encryption)** | ✅ Native SDES | Traditional SDES-SRTP, matching FlowPBX's RTP media proxy architecture. |
| **Opus codec** | ✅ | Native codec support including Opus, G.711 (PCMU/PCMA), G.722, and others. |
| **Push wake-up** | ✅ Built-in | PushKit + CallKit on iOS, FCM on Android. SDK handles push token lifecycle. |
| **G.711 (PCMU/PCMA)** | ✅ | Native codec support. |
| **DTMF** | ✅ | RFC 2833 and SIP INFO. |
| **Background calls** | ✅ | Native audio session management. Push-to-wake architecture built in. |
| **CallKit / ConnectionService** | ✅ | Native CallKit (iOS) and ConnectionService (Android) integration included. |
| **Platforms** | ✅ | Android, iOS, macOS, Windows, Linux. |

**Pros**: Best feature coverage of any Flutter SIP option. Native SIP/RTP stack matches FlowPBX architecture perfectly (TLS + SDES-SRTP). Push notifications, CallKit, and background operation all built in. Multi-platform support. Actively maintained.

**Cons**: **Commercial license required** — trial drops calls after 60 seconds. One-time fee (not subscription), but exact pricing requires contacting sales@siprix-voip.com. Adds a proprietary binary dependency to the app. Native SDK is closed-source.

**Verdict**: Excellent technical fit for FlowPBX. The commercial license is the main drawback for a self-hosted open-source PBX product. **Recommended if commercial dependency is acceptable.**

##### 2. flutter_pjsip (PJSIP via Dart FFI)

**Package**: [flutter_pjsip](https://pub.dev/packages/flutter_pjsip) v0.0.2-dev.2
**GitHub**: [JackZhang1994/flutter_pjsip](https://github.com/JackZhang1994/flutter_pjsip)
**Last updated**: January 2024 (>2 years stale)
**License**: BSD-3-Clause

| Criterion | Support | Notes |
|---|---|---|
| **TLS (SIP signaling)** | ✅ Via PJSIP | PJSIP supports TLS natively, but this wrapper's coverage is undocumented. |
| **SRTP (media encryption)** | ✅ Via PJSIP | PJSIP supports SDES-SRTP natively, but wrapper exposure is undocumented. |
| **Opus codec** | ✅ Via PJSIP | PJSIP supports Opus when compiled with opus support. |
| **Push wake-up** | ❌ Not included | No push notification integration. Would need custom implementation. |
| **CallKit / ConnectionService** | ❌ Not included | No native call UI integration. |
| **Background calls** | ❓ Unknown | Depends on PJSIP background handling, undocumented in wrapper. |

**Pros**: PJSIP is battle-tested and fully supports traditional SIP/RTP with TLS and SDES-SRTP. Open-source. If the FFI bindings worked well, this would be the ideal open-source option.

**Cons**: **Effectively abandoned** — pre-release v0.0.2-dev, last updated January 2024, unverified publisher, minimal documentation ("API docs to be available later"). Dart FFI bindings are auto-generated but incomplete. Would require major effort to bring to production quality. Requires Android NDK setup. No push/CallKit integration.

**Verdict**: Interesting approach but **not viable** in current state. Would require forking and significant investment to make production-ready. Not recommended.

##### 3. sip_native

**Package**: Not on pub.dev (GitHub only)
**GitHub**: [iampato/sip_native](https://github.com/iampato/sip_native)
**Last updated**: ~2021 (stale)
**License**: Not specified

| Criterion | Support | Notes |
|---|---|---|
| **TLS (SIP signaling)** | ✅ | Supports UDP, TLS, TCP transport selection. |
| **SRTP (media encryption)** | ❌ Not documented | No mention of SRTP support. |
| **Opus codec** | ❌ Not documented | Relies on Android's deprecated `android.net.sip` API — limited codec support. |
| **Push wake-up** | ❌ Not included | No push notification integration. |
| **CallKit / ConnectionService** | ❌ Not included | No native call UI integration. |
| **Background calls** | ❓ Unknown | Undocumented. |
| **Platforms** | ⚠️ Android only | iOS not supported despite Flutter being cross-platform. |

**Pros**: Uses native Android SIP API, supports TLS transport.

**Cons**: **Android-only**, no iOS support. Uses deprecated `android.net.sip` API (removed in newer Android versions). No SRTP. Stale project (~2021). Missing critical features. Not on pub.dev.

**Verdict**: **Not viable.** Android-only, deprecated API, missing core requirements. Not recommended.

##### 4. ABTO Software VoIP SIP SDK

**Package**: Not on pub.dev (commercial distribution)
**Website**: [voipsipsdk.com](https://voipsipsdk.com/products/voip-sip-sdk-for-flutter/features)
**Last updated**: August 2025 (iOS SDK 20250805, Android SDK 20250822)
**License**: Commercial

| Criterion | Support | Notes |
|---|---|---|
| **TLS (SIP signaling)** | ✅ | TLS 1.3 support on iOS (using OpenSSL 1.1.1w). |
| **SRTP (media encryption)** | ✅ | libSRTP 2.5.0 on both platforms. |
| **Opus codec** | ✅ | Full codec support. |
| **Push wake-up** | ✅ | PushKit (iOS) + FCM (Android). |
| **CallKit / ConnectionService** | ✅ | Native integration. |

**Pros**: Full-featured, actively maintained, production-grade SIP SDK with TLS 1.3 and SRTP.

**Cons**: **Commercial license** — pricing not public, requires contacting vendor. Not distributed via pub.dev. Less community visibility than Siprix. Adds proprietary binary dependency.

**Verdict**: Strong technical fit but **same commercial licensing concern** as Siprix, with less community visibility. Not recommended over Siprix.

##### Native Bridge Summary

| Library | Architecture Match | Completeness | Maintenance | License | Recommendation |
|---|---|---|---|---|---|
| **siprix_voip_sdk** | ✅ Excellent | ✅ Full | ✅ Active | ⚠️ Commercial | Best option if commercial OK |
| **ABTO VoIP SDK** | ✅ Excellent | ✅ Full | ✅ Active | ⚠️ Commercial | Alternative commercial option |
| **flutter_pjsip** | ✅ Good (PJSIP) | ❌ Incomplete | ❌ Stale | ✅ Open source | Not viable in current state |
| **sip_native** | ⚠️ Partial | ❌ Minimal | ❌ Stale | ✅ Open source | Not viable |

### Authentication & Registration
- [ ] Create login screen: server URL + extension number + SIP password
- [ ] Implement app auth flow: call `POST /api/v1/app/auth` → store JWT + SIP config
- [ ] Implement secure token storage (flutter_secure_storage)
- [ ] Implement SIP registration to PBX over TLS/TCP using credentials from auth response
- [ ] Implement auto-reconnect on network change (WiFi ↔ cellular)
- [ ] Implement registration status indicator in UI
- [ ] Implement logout: de-register SIP, clear tokens, return to login
