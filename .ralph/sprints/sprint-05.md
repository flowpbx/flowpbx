# Sprint 05 — SIP Stack Initialization

**Phase**: 1A (Foundation)
**Focus**: sipgo setup, SIP listeners, REGISTER handler, registration management
**Dependencies**: Sprint 02

**PRD Reference**: Section 8.1 (Transports), Section 8.3 (Inbound Call Handling), Section 4.12 (Registrations)

## Tasks

- [ ] Add sipgo dependency (`github.com/emiago/sipgo`)
- [ ] Create `internal/sip/server.go` — sipgo UA + Server setup, start/stop lifecycle
- [ ] Configure UDP listener on configurable port (default 5060)
- [ ] Configure TCP listener on configurable port (default 5060)
- [ ] Add TLS listener support on configurable port (default 5061, requires cert)
- [ ] Reserve WSS listener config (port 8089, disabled by default, for Phase 2 WebRTC)
- [ ] Create `internal/sip/auth.go` — SIP digest authentication against extensions table
- [ ] Create `internal/sip/registrar.go` — REGISTER handler: authenticate, store contact in registrations table
- [ ] Handle multiple registrations per extension (desk phone + mobile + softphone, up to max_registrations)
- [ ] Store push token and device_id from REGISTER Contact parameters
- [ ] Create registration expiry cleanup goroutine (remove expired registrations periodically)
- [ ] Implement SIP OPTIONS responder (respond to OPTIONS pings from trunks/phones)
- [ ] Add structured SIP message logging (configurable verbosity via log level)
- [ ] Wire SIP server startup into main.go (start after DB init)
