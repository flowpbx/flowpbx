# FlowPBX - Build & Test Commands

## Go Backend

### Build
```bash
go build -o bin/flowpbx ./cmd/flowpbx
go build -o bin/pushgw ./cmd/pushgw
```

### Run
```bash
go run ./cmd/flowpbx
go run ./cmd/flowpbx --sip-port 5060 --http-port 8080 --data-dir ./data
go run ./cmd/pushgw
```

### Test
```bash
go test ./...
go test -cover ./...
go test -race ./...
```

### Lint
```bash
gofmt -w .
go vet ./...
golangci-lint run
```

## React Admin UI

```bash
cd web/
npm install
npm run dev          # dev server with HMR
npm run build        # production build → web/dist/
npm run lint         # eslint
```

## Flutter Mobile App

```bash
cd mobile/
flutter pub get
flutter run              # run on connected device/emulator
flutter build apk        # Android release
flutter build ios        # iOS release
flutter test             # run tests
flutter analyze          # lint
```

## Full Build (Go + Embedded UI)

```bash
make build           # builds UI then Go binary
make dev             # dev mode (separate servers)
make test            # all tests
make lint            # go + js lint
make release         # cross-compile linux/amd64 + arm64
```

## Environment

All config via env vars with `FLOWPBX_` prefix or CLI flags:
- `FLOWPBX_HTTP_PORT` - Admin UI port (default: 8080)
- `FLOWPBX_SIP_UDP_PORT` - SIP UDP port (default: 5060)
- `FLOWPBX_SIP_TCP_PORT` - SIP TCP port (default: 5060)
- `FLOWPBX_SIP_TLS_PORT` - SIP TLS port (default: 5061)
- `FLOWPBX_DATA_DIR` - Data directory (default: ./data)
- `FLOWPBX_EXTERNAL_IP` - Public IP for SIP/RTP
- `FLOWPBX_LOG_LEVEL` - Logging level (default: info)
- `FLOWPBX_LICENSE_KEY` - License key
- `FLOWPBX_PUSH_GATEWAY_URL` - Push gateway URL

## Database

SQLite with WAL mode. Database file at `$DATA_DIR/flowpbx.db`.
Migrations are embedded SQL files, run automatically on startup.

## Project Structure

```
flowpbx/
├── cmd/flowpbx/           # PBX entry point
├── cmd/pushgw/            # Push gateway / license server entry point
├── internal/
│   ├── config/            # Config loading
│   ├── database/          # SQLite, migrations, models
│   ├── api/               # HTTP handlers (chi router)
│   │   ├── middleware/    # Auth, logging, CORS
│   │   ├── admin/         # Admin API handlers
│   │   ├── app/           # Mobile app API handlers
│   │   └── ws/            # WebSocket handlers
│   ├── sip/               # SIP engine (sipgo)
│   ├── media/             # RTP proxy, codecs, recording
│   ├── flow/              # Call flow engine + node handlers
│   ├── voicemail/         # Voicemail management
│   ├── recording/         # Recording management
│   ├── push/              # Push gateway client
│   └── license/           # License validation
├── web/                   # React admin UI (Vite + Tailwind + React Flow)
├── migrations/            # SQL migration files
├── prompts/               # Default audio prompts (embedded)
├── mobile/                # Flutter softphone app (iOS + Android)
│   ├── lib/               # Dart source
│   │   ├── screens/       # UI screens
│   │   ├── services/      # SIP, API, push services
│   │   ├── models/        # Data models
│   │   ├── widgets/       # Reusable widgets
│   │   └── providers/     # State management
│   ├── ios/               # iOS native config
│   ├── android/           # Android native config
│   └── pubspec.yaml
├── Makefile
├── go.mod
└── go.sum
```
