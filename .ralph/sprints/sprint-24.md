# Sprint 24 — Polish & Hardening

**Phase**: 1F (Polish & Hardening)
**Focus**: Testing, security, monitoring, documentation, operational readiness
**Dependencies**: All previous sprints

**PRD Reference**: Section 13 Phase 1F, Section 14 (Non-Functional Requirements), Section 15 (Risks)

## Tasks

### Testing
- [ ] Go unit tests for core packages (target 70%+ coverage)
- [ ] SIP integration tests (automated call testing)
- [ ] Flow engine tests: validate each node type with mock calls
- [ ] API endpoint tests for all CRUD operations
- [ ] UI component tests (React Testing Library)
- [ ] Flutter widget and integration tests
- [ ] Load testing: 50 concurrent calls on minimum spec hardware
- [ ] Trunk failover testing
- [ ] Voicemail box tests: recording, MWI, email notification, retention cleanup
- [ ] Push testing: app backgrounded → push → wake → answer (both platforms)

### Security Hardening
- [ ] SIP auth: nonce replay prevention, brute-force lockout
- [ ] Fail2ban-style IP blocking for failed SIP auth attempts
- [x] Rate limiting on all API endpoints
- [ ] HTTPS enforcement for admin UI (auto Let's Encrypt or manual cert)
- [ ] Verify all secrets encrypted at rest (SIP passwords, trunk credentials)
- [ ] Input validation on all API endpoints
- [ ] Verify parameterized queries only (no SQL injection vectors)
- [ ] CSRF protection on admin UI
- [x] Security headers (CSP, HSTS, X-Frame-Options, etc.)

### Monitoring & Observability
- [ ] Structured JSON logging throughout (configurable level)
- [ ] SIP message logging (configurable verbosity)
- [ ] Prometheus metrics endpoint `/metrics` (active calls, registrations, trunk status, call volume, RTP stats)
- [ ] WebSocket endpoint `/ws` for real-time admin dashboard updates (active calls, registrations, trunk status)
- [ ] Wire dashboard page to WebSocket for live stats
- [ ] Implement `GET /api/v1/calls/active` — list active calls (REST fallback)
- [ ] Implement `POST /api/v1/calls/:id/hangup` — admin hangup
- [ ] Implement `POST /api/v1/calls/:id/transfer` — admin transfer

### Operational
- [ ] Graceful shutdown: drain active calls, de-register trunks, close DB
- [ ] Config hot-reload (`POST /api/v1/system/reload`): reload flows, extensions without restart
- [ ] Log rotation guidance
- [ ] Automatic cleanup of old recordings (configurable retention)
- [ ] Automatic cleanup of old voicemail messages (per-box retention_days)
- [ ] Startup self-test: verify ports available, external IP reachable, DNS resolution
- [ ] Version check against push gateway (notify admin of updates)

### Documentation
- [ ] README with quickstart guide
- [ ] Admin guide: installation, configuration, trunk setup
- [ ] User guide: mobile app setup, voicemail, follow-me
- [ ] API documentation (OpenAPI / Swagger spec)
- [ ] Troubleshooting guide (common SIP issues)
