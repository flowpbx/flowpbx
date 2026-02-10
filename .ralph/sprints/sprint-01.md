# Sprint 01 — Project Scaffolding

**Phase**: 1A (Foundation)
**Focus**: Go module, repo structure, build pipeline, Makefile
**Dependencies**: None

**PRD Reference**: Section 16 (Go Project Structure), Section 12 (Deployment & Build)

## Tasks

- [ ] Initialize Go module (`github.com/flowpbx/flowpbx`)
- [ ] Create directory structure: `cmd/flowpbx/`, `cmd/pushgw/`, `internal/config/`, `internal/database/`, `internal/api/`, `internal/sip/`, `internal/media/`, `internal/flow/`, `internal/voicemail/`, `internal/recording/`, `internal/push/`, `internal/license/`, `internal/pushgw/`
- [ ] Create `cmd/flowpbx/main.go` entry point (boots HTTP server, placeholder SIP init)
- [ ] Create `cmd/pushgw/main.go` entry point (boots push gateway HTTP server)
- [ ] Create Makefile with targets: `build`, `dev`, `test`, `lint`, `ui-build`, `release`
- [ ] Set up cross-compilation in Makefile for linux/amd64 and linux/arm64
- [ ] Create `.github/workflows/ci.yml` — lint + test on PR
- [ ] Create `.github/workflows/release.yml` — build + release on tag
- [ ] Create `internal/config/config.go` — load config from CLI flags, env vars (`FLOWPBX_` prefix), and defaults
- [ ] Implement config precedence: CLI flags > env vars > database config > defaults
- [ ] Add `--data-dir`, `--http-port`, `--sip-port`, `--sip-tls-port`, `--tls-cert`, `--tls-key`, `--log-level` flags
