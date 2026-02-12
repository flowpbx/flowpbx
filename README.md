# FlowPBX

> **WARNING: This project is under active development and is NOT production-ready.**
> APIs, database schemas, and configuration formats may change without notice. Use at your own risk. We're building in the open — contributions and feedback are welcome, but please do not deploy this for real phone systems yet.

A single-binary, self-hosted PBX for small-to-medium businesses (5–100 extensions). FlowPBX combines a visual drag-and-drop call flow editor with a complete SIP telephony system — if you can draw a flowchart, you can build a phone system.

## License

FlowPBX is licensed under the [Business Source License 1.1](LICENSE) (BUSL-1.1):

- **Free for small deployments** — Use FlowPBX in production with up to **5 SIP extensions** at no cost
- **Source available** — Full source code is available for inspection, modification, and redistribution
- **Commercial license required** — More than 5 extensions requires purchasing a commercial license
- **Converts to open source** — On **2030-02-11** (or 4 years from first public release), FlowPBX automatically converts to **AGPLv3**

This licensing model allows small businesses and personal use cases to benefit from FlowPBX for free, while larger deployments support continued development.

## Features

- **Visual Call Flow Editor** — Drag-and-drop canvas (React Flow) to build call routing logic with nodes for extensions, ring groups, IVR menus, time switches, voicemail, conferences, and more
- **Single Binary** — Go binary with embedded React admin UI, SQLite database, no external dependencies
- **Full SIP Server** — UDP, TCP, and TLS transports with digest authentication, registration, and IP-auth trunks
- **RTP Media Proxy** — G.711 and Opus codecs, call recording, conference mixing, DTMF detection
- **Voicemail** — Custom greetings, email notifications, MWI, browser playback
- **Ring Groups** — Ring all, round-robin, random, and longest-idle strategies
- **Follow-Me** — Sequential or simultaneous ringing to external numbers
- **IVR Menus** — DTMF collection and multi-level routing
- **Time-Based Routing** — Timezone-aware schedules for business hours, holidays, etc.
- **Conference Bridges** — Multi-party audio mixing with participant management
- **Call Recording** — Per-extension and per-trunk policies
- **CDR & Metrics** — Call detail records with CSV export, Prometheus `/metrics` endpoint
- **Mobile App** — Flutter softphone with push notifications, CallKit/ConnectionService integration
- **Push Gateway** — Centralized FCM/APNs delivery for mobile wake-up on incoming calls

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go, `sipgo` (SIP), `chi` (HTTP), SQLite (WAL mode) |
| Frontend | React 18, TypeScript, Vite, Tailwind CSS, XY Flow |
| Mobile | Flutter, Riverpod, Siprix VoIP SDK |
| Push Gateway | Go, PostgreSQL, Firebase Admin SDK |
| Auth | JWT (mobile), session cookies (admin), SIP digest |
| Encryption | AES-256-GCM for sensitive fields |

## Project Structure

```
flowpbx/
├── cmd/
│   ├── flowpbx/          # PBX server entry point
│   └── pushgw/           # Push gateway entry point
├── internal/
│   ├── api/              # REST API handlers and middleware
│   ├── sip/              # SIP engine (registrar, invite, trunks, auth)
│   ├── media/            # RTP proxy, codecs, mixer, recorder
│   ├── flow/             # Call flow graph engine
│   ├── database/         # SQLite migrations and repositories
│   ├── config/           # Configuration (CLI/env/db)
│   ├── voicemail/        # Voicemail management
│   ├── recording/        # Recording management
│   ├── email/            # SMTP notifications
│   ├── prompts/          # Audio prompt system
│   ├── push/             # Push gateway client
│   ├── metrics/          # Prometheus metrics
│   └── license/          # License validation
├── web/                  # React admin UI
├── mobile/               # Flutter softphone app
├── data/                 # Runtime data (DB, recordings, voicemail)
└── Makefile
```

## Prerequisites

- Go 1.25+
- Node.js 20 LTS (for web UI build)
- Flutter 3.2+ (for mobile app, optional)
- PostgreSQL (for push gateway only, optional)

## Quick Start

```bash
# Build everything (UI + server binaries)
make build

# Run in development mode
make dev

# Run the built binary
./build/flowpbx --data-dir ./data
```

The admin UI is available at `http://localhost:8080` by default.

## Configuration

Configuration follows CLI flags > environment variables > defaults. All environment variables use the `FLOWPBX_` prefix.

| Variable | Default | Description |
|----------|---------|-------------|
| `FLOWPBX_DATA_DIR` | `./data` | Database and file storage |
| `FLOWPBX_HTTP_PORT` | `8080` | Admin UI and API port |
| `FLOWPBX_SIP_PORT` | `5060` | SIP UDP/TCP port |
| `FLOWPBX_SIP_TLS_PORT` | `5061` | SIP TLS port |
| `FLOWPBX_RTP_PORT_MIN` | `10000` | RTP port range start |
| `FLOWPBX_RTP_PORT_MAX` | `20000` | RTP port range end |
| `FLOWPBX_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `FLOWPBX_LOG_FORMAT` | `text` | `text` or `json` |
| `FLOWPBX_EXTERNAL_IP` | (auto) | Public IP for SDP rewriting |
| `FLOWPBX_ENCRYPTION_KEY` | — | Hex 32-byte AES key for password encryption |
| `FLOWPBX_ACME_DOMAIN` | — | Domain for automatic Let's Encrypt TLS |
| `FLOWPBX_ACME_EMAIL` | — | Email for Let's Encrypt notifications |
| `FLOWPBX_TLS_CERT` | — | Path to TLS certificate (manual) |
| `FLOWPBX_TLS_KEY` | — | Path to TLS private key (manual) |
| `FLOWPBX_PUSH_GATEWAY_URL` | — | URL of push gateway service |
| `FLOWPBX_JWT_SECRET` | (auto) | Hex 32-byte secret for mobile JWT |
| `FLOWPBX_CORS_ORIGINS` | — | Comma-separated CORS origins |

## TLS

```bash
# Automatic via Let's Encrypt
./build/flowpbx --acme-domain pbx.example.com --acme-email admin@example.com

# Manual certificate
./build/flowpbx --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem
```

## Data Directory

```
data/
├── flowpbx.db          # SQLite database
├── recordings/         # Call recordings (date-organized)
├── voicemail/          # Voicemail messages (per-box)
├── prompts/            # IVR and system audio prompts
├── greetings/          # Voicemail greetings
└── acme-certs/         # Let's Encrypt certificates
```

## Mobile App

```bash
cd mobile
flutter pub get
flutter build apk --release      # Android
flutter build ios --release       # iOS (macOS only)
```

The mobile app connects to the PBX server and supports push notifications for incoming calls via the push gateway.

## Push Gateway

A separate service for centralized push notification delivery (FCM/APNs):

```bash
./build/pushgw \
  --db-dsn "postgres://user:pass@localhost/pushgw" \
  --fcm-credentials /path/to/firebase-key.json
```

## Make Targets

```
make build          Build flowpbx and pushgw binaries
make dev            Run in development mode with race detector
make test           Run Go tests with race detector
make lint           Run golangci-lint and go vet
make ui-build       Build React admin UI
make mobile-build   Build Flutter APK + iOS
make release        Cross-compile for linux/amd64 and linux/arm64
make clean          Remove build artifacts
make help           Show all targets
```

## Commercial Licensing

If you need more than 5 extensions, commercial licenses are available. Commercial licenses include:

- Unlimited extensions
- Priority support
- Commercial license terms suitable for enterprises
- Optional professional services and consulting

For commercial licensing inquiries, please open an issue on GitHub or contact the project maintainers.

## Contributing

Contributions are welcome! Since FlowPBX uses the Business Source License, contributions will be subject to the same license terms. By contributing, you agree that your contributions will be licensed under BUSL-1.1 and will convert to AGPLv3 on the Change Date.
