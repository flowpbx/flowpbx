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
- [ ] Evaluate `dart_sip_ua` — TLS, SRTP, Opus support, push wake-up capability
- [ ] Evaluate `flutter_ooh_sip` / native bridge options
- [ ] Evaluate fallback: native iOS (Swift) + Android (Kotlin) SIP engines with Flutter UI via platform channels
- [ ] Select library and document decision in PROMPT_learnings.md

### Authentication & Registration
- [ ] Create login screen: server URL + extension number + SIP password
- [ ] Implement app auth flow: call `POST /api/v1/app/auth` → store JWT + SIP config
- [ ] Implement secure token storage (flutter_secure_storage)
- [ ] Implement SIP registration to PBX over TLS/TCP using credentials from auth response
- [ ] Implement auto-reconnect on network change (WiFi ↔ cellular)
- [ ] Implement registration status indicator in UI
- [ ] Implement logout: de-register SIP, clear tokens, return to login
