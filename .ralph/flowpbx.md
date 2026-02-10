# [PROJECT CODENAME: FLOWPBX] — Product Requirements Document

**Owner:** The IT Dept Pty Ltd (ABN 12 665 405 505)
**Version:** 1.1
**Date:** 2026-02-10
**Status:** Draft — Awaiting Sign-off

---

## 1. Vision

A single-binary, self-hosted PBX system for small-to-medium businesses (5–100 extensions) that replaces bloated legacy PBX platforms with a modern, visual call flow editor. The entire system — SIP server, media proxy, admin UI, and API — ships as one Go binary. Call routing logic is built visually using a React Flow drag-and-drop graph editor. A companion Flutter softphone app connects users on iOS and Android. A centrally-hosted push gateway doubles as a license/activation server.

**Core thesis:** Office PBX should not be hard. If you can draw a flowchart, you can build a phone system.

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                     FLOWPBX BINARY                      │
│                                                         │
│  ┌──────────┐  ┌──────────┐  ┌────────────────────────┐ │
│  │ SIP Stack│  │ RTP/Media│  │   HTTP Server (chi)    │ │
│  │ (sipgo)  │◄►│  Proxy   │  │  ┌──────────────────┐  │ │
│  │          │  │          │  │  │ Admin API (REST)  │  │ │
│  │ UDP/TCP/ │  │ G.711    │  │  ├──────────────────┤  │ │
│  │ TLS/WSS  │  │ Opus     │  │  │ Embedded React   │  │ │
│  │          │  │ Transcode│  │  │ SPA (static)     │  │ │
│  │ Register │  │ Record   │  │  └──────────────────┘  │ │
│  │ & IP Auth│  │ Bridge   │  │                        │ │
│  └──────────┘  └──────────┘  └────────────────────────┘ │
│  ┌──────────────────────────────────────────────────┐   │
│  │              SQLite (WAL mode)                   │   │
│  │  Config │ Registrations │ CDRs │ Voicemail Meta  │   │
│  └──────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────┐   │
│  │           Local Filesystem Storage               │   │
│  │     Voicemail WAVs │ Call Recordings │ Prompts   │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
         ▲                          ▲
         │ SIP/RTP                  │ HTTPS
         ▼                          ▼
┌─────────────────┐      ┌──────────────────┐
│  SIP Trunks     │      │  Flutter App     │
│  (Register/IP)  │      │  (iOS/Android)   │
│                 │      │                  │
│  Desk Phones    │      │  ◄── Push via ──►│
│  (any SIP)      │      │   Push Gateway   │
└─────────────────┘      └──────────────────┘
                                  ▲
                                  │ HTTPS/FCM/APNs
                                  ▼
                         ┌──────────────────┐
                         │  Push Gateway    │
                         │  (Multi-tenant)  │
                         │  + License Server│
                         │  Hosted by TITD  │
                         └──────────────────┘
```

---

## 3. Technology Stack

- [ ] **Language:** Go 1.22+ — single binary compilation
- [ ] **SIP Stack:** `sipgo` (github.com/emiago/sipgo) — RFC 3261 compliant, fast parser
- [ ] **HTTP Router:** `chi` (github.com/go-chi/chi) — lightweight, middleware-friendly
- [ ] **Database:** SQLite 3 with WAL mode — `github.com/mattn/go-sqlite3` (CGO) or `modernc.org/sqlite` (pure Go)
- [ ] **Admin UI:** React 18 + Tailwind CSS + React Flow — embedded as static assets via `embed`
- [ ] **Mobile App:** Flutter (Dart) — single codebase iOS + Android
- [ ] **SIP in Flutter:** Evaluate `dart_sip_ua` / native bridge — must support TLS, SRTP, Opus, push wake-up
- [ ] **Media:** Custom RTP proxy — G.711a/u, Opus transcoding
- [ ] **Build:** GitHub Actions — cross-compile Linux/amd64, arm64
- [ ] **Push Gateway:** Go service (`cmd/pushgw`) — FCM + APNs, multi-tenant, same repo

---

## 4. Data Model (Core Entities)

### 4.1 System Config
```
system_config {
  id           INTEGER PRIMARY KEY
  key          TEXT UNIQUE        -- e.g. "sip.udp_port", "sip.tcp_port"
  value        TEXT
  updated_at   DATETIME
}
```

- [ ] Key-value config store for all system settings
- [ ] Loaded into memory on startup, refreshed on change
- [ ] Encrypted value support for sensitive keys

### 4.2 Trunks
```
trunks {
  id              INTEGER PRIMARY KEY
  name            TEXT NOT NULL
  type            TEXT NOT NULL        -- "register" | "ip"
  enabled         BOOLEAN DEFAULT 1

  -- Registration trunk fields
  host            TEXT                 -- SIP registrar host
  port            INTEGER DEFAULT 5060
  transport       TEXT DEFAULT "udp"   -- udp | tcp | tls
  username        TEXT
  password        TEXT                 -- encrypted at rest
  auth_username   TEXT                 -- if different from username
  register_expiry INTEGER DEFAULT 300
  
  -- IP auth trunk fields
  remote_hosts    TEXT                 -- JSON array of allowed IPs/CIDRs
  local_host      TEXT                 -- optional bind address
  
  -- Common
  codecs          TEXT                 -- JSON array: ["g711a","g711u","opus"]
  max_channels    INTEGER DEFAULT 0   -- 0 = unlimited
  caller_id_name  TEXT
  caller_id_num   TEXT
  prefix_strip    INTEGER DEFAULT 0
  prefix_add      TEXT DEFAULT ""
  priority        INTEGER DEFAULT 10  -- lower = preferred for outbound
  
  created_at      DATETIME
  updated_at      DATETIME
}
```

- [ ] Support both registration-based and IP-auth trunks
- [ ] Encrypted password storage (AES-256-GCM)
- [ ] Priority field for outbound trunk selection

### 4.3 Inbound Numbers (DIDs)
```
inbound_numbers {
  id              INTEGER PRIMARY KEY
  number          TEXT NOT NULL        -- E.164 format
  name            TEXT                 -- friendly label "Main Office"
  trunk_id        INTEGER REFERENCES trunks(id)
  flow_id         INTEGER REFERENCES call_flows(id)
  flow_entry_node TEXT                 -- node ID in the flow graph
  enabled         BOOLEAN DEFAULT 1
  created_at      DATETIME
  updated_at      DATETIME
}
```

- [ ] E.164 number format validation
- [ ] Maps to a specific entry node in a specific flow
- [ ] Multiple numbers can map to the same entry node

### 4.4 Extensions / Users
```
extensions {
  id              INTEGER PRIMARY KEY
  extension       TEXT NOT NULL UNIQUE -- "101", "102"
  name            TEXT NOT NULL        -- "Nick"
  email           TEXT
  
  -- SIP credentials
  sip_username    TEXT NOT NULL UNIQUE
  sip_password    TEXT NOT NULL        -- bcrypt or argon2
  
  -- Settings
  ring_timeout       INTEGER DEFAULT 30
  dnd                BOOLEAN DEFAULT 0
  
  -- Follow-me
  follow_me_enabled  BOOLEAN DEFAULT 0
  follow_me_numbers  TEXT              -- JSON: [{"number":"0412345678","delay":10,"timeout":20}]
  
  -- Recording
  recording_mode     TEXT DEFAULT "off" -- "off" | "always" | "on_demand"
  
  -- Device tracking
  max_registrations  INTEGER DEFAULT 5
  
  created_at      DATETIME
  updated_at      DATETIME
}
```

- [ ] Extension number uniqueness enforcement
- [ ] SIP credential generation (auto-generate secure password option)
- [ ] Follow-me as JSON array of external numbers with delay/timeout
- [ ] Recording mode per extension

### 4.5 Voicemail Boxes
```
voicemail_boxes {
  id                 INTEGER PRIMARY KEY
  name               TEXT NOT NULL        -- "Sales Voicemail", "Nick's Voicemail"
  mailbox_number     TEXT UNIQUE          -- optional dial-in number e.g. "901"
  pin                TEXT                 -- access PIN, hashed
  
  -- Greeting
  greeting_file      TEXT                 -- path to custom greeting WAV
  greeting_type      TEXT DEFAULT "default" -- "default" | "custom" | "name_only"
  
  -- Notifications
  email_notify       BOOLEAN DEFAULT 0
  email_address      TEXT                 -- notification recipient
  email_attach_audio BOOLEAN DEFAULT 1   -- attach WAV to email
  
  -- Settings
  max_message_duration INTEGER DEFAULT 120 -- seconds
  max_messages         INTEGER DEFAULT 50
  retention_days       INTEGER DEFAULT 90  -- auto-delete after N days, 0 = forever
  
  -- Optional extension association (for MWI)
  notify_extension_id  INTEGER REFERENCES extensions(id)
  
  created_at      DATETIME
  updated_at      DATETIME
}
```

- [ ] Voicemail boxes are independent entities, not tied to extensions
- [ ] Can be used as flow destinations (e.g., "Sales VM", "After Hours VM", "Personal VM")
- [ ] Optional association with an extension for MWI (message waiting indicator) lamp
- [ ] PIN-protected access for checking messages
- [ ] Optional dial-in mailbox number for direct access
- [ ] Email notification with optional audio attachment
- [ ] Configurable retention and message limits

### 4.6 Voicemail Messages
```
voicemail_messages {
  id              INTEGER PRIMARY KEY
  mailbox_id      INTEGER NOT NULL REFERENCES voicemail_boxes(id)
  caller_id_name  TEXT
  caller_id_num   TEXT
  timestamp       DATETIME NOT NULL
  duration        INTEGER
  file_path       TEXT NOT NULL
  read            BOOLEAN DEFAULT 0
  read_at         DATETIME
  transcription   TEXT                 -- future: speech-to-text
  created_at      DATETIME
}
```

- [ ] Messages belong to a voicemail box, not an extension
- [ ] Track read status and read timestamp
- [ ] File path to WAV recording
- [ ] Transcription field for future STT integration

### 4.7 Ring Groups
```
ring_groups {
  id              INTEGER PRIMARY KEY
  name            TEXT NOT NULL
  strategy        TEXT DEFAULT "ring_all"  -- ring_all | round_robin | random | longest_idle
  ring_timeout    INTEGER DEFAULT 30
  members         TEXT NOT NULL         -- JSON array of extension IDs
  caller_id_mode  TEXT DEFAULT "pass"   -- pass | fixed | prepend
  created_at      DATETIME
  updated_at      DATETIME
}
```

- [ ] Multiple ring strategies
- [ ] Configurable timeout before failover
- [ ] Caller ID pass-through or override

### 4.8 IVR Menus
```
ivr_menus {
  id              INTEGER PRIMARY KEY
  name            TEXT NOT NULL
  greeting_file   TEXT                  -- path to audio prompt
  greeting_tts    TEXT                  -- TTS fallback text
  timeout         INTEGER DEFAULT 10   -- seconds to wait for input
  max_retries     INTEGER DEFAULT 3
  digit_timeout   INTEGER DEFAULT 3    -- inter-digit timeout
  options         TEXT NOT NULL         -- JSON: {"1":"node_xxx","2":"node_yyy","i":"node_timeout","t":"node_invalid"}
  created_at      DATETIME
  updated_at      DATETIME
}
```

- [ ] DTMF digit mapping to flow edges
- [ ] Configurable timeouts and retry counts
- [ ] Audio prompt upload or TTS fallback

### 4.9 Time Switches
```
time_switches {
  id              INTEGER PRIMARY KEY
  name            TEXT NOT NULL
  timezone        TEXT DEFAULT "Australia/Sydney"
  rules           TEXT NOT NULL         -- JSON array of time rules
  -- rule format: {"label":"Business Hours","days":["mon","tue","wed","thu","fri"],"start":"08:30","end":"17:00","dest_node":"node_xxx"}
  -- evaluated top to bottom, first match wins
  default_dest    TEXT                  -- node ID for no-match (after hours)
  created_at      DATETIME
  updated_at      DATETIME
}
```

- [ ] Timezone-aware rule evaluation
- [ ] Top-to-bottom rule matching, first wins
- [ ] Support for holiday/specific date overrides

### 4.10 Call Flow Graph
```
call_flows {
  id              INTEGER PRIMARY KEY
  name            TEXT NOT NULL         -- "Main Inbound Flow"
  flow_data       TEXT NOT NULL         -- Full React Flow JSON (nodes + edges)
  version         INTEGER DEFAULT 1
  published       BOOLEAN DEFAULT 0    -- draft vs live
  published_at    DATETIME
  created_at      DATETIME
  updated_at      DATETIME
}
```

- [ ] Flow data stored as React Flow JSON (nodes + edges)
- [ ] Draft vs published versioning
- [ ] Multiple flows supported (one per inbound number group, or reusable)

### 4.11 CDR (Call Detail Records)
```
cdrs {
  id              INTEGER PRIMARY KEY
  call_id         TEXT NOT NULL         -- SIP Call-ID
  start_time      DATETIME NOT NULL
  answer_time     DATETIME
  end_time        DATETIME
  duration        INTEGER              -- total seconds
  billable_dur    INTEGER              -- answered seconds
  caller_id_name  TEXT
  caller_id_num   TEXT
  callee          TEXT
  trunk_id        INTEGER
  direction       TEXT                 -- "inbound" | "outbound" | "internal"
  disposition     TEXT                 -- "answered" | "no_answer" | "busy" | "failed" | "voicemail"
  recording_file  TEXT                 -- path if recorded
  flow_path       TEXT                 -- JSON array of node IDs traversed
  hangup_cause    TEXT
}
```

- [ ] Created on call start, updated on answer and hangup
- [ ] Flow traversal path recorded for debugging
- [ ] Recording file reference

### 4.12 Active Registrations (runtime, ephemeral)
```
registrations {
  id              INTEGER PRIMARY KEY
  extension_id    INTEGER REFERENCES extensions(id)
  contact_uri     TEXT NOT NULL        -- full SIP contact
  transport       TEXT                 -- udp | tcp | tls | wss
  user_agent      TEXT
  source_ip       TEXT
  source_port     INTEGER
  expires         DATETIME
  registered_at   DATETIME
  push_token      TEXT                 -- FCM/APNs token for mobile
  push_platform   TEXT                 -- "fcm" | "apns"
  device_id       TEXT                 -- unique device identifier
}
```

- [ ] Ephemeral — cleaned up on expiry and restart
- [ ] Push token storage for mobile wake-up
- [ ] Multiple registrations per extension (desk phone + mobile + softphone)

### 4.13 Conference Bridges
```
conference_bridges {
  id              INTEGER PRIMARY KEY
  name            TEXT NOT NULL
  extension       TEXT UNIQUE          -- dial-in extension
  pin             TEXT                 -- optional access PIN
  max_members     INTEGER DEFAULT 10
  record          BOOLEAN DEFAULT 0
  mute_on_join    BOOLEAN DEFAULT 0
  announce_joins  BOOLEAN DEFAULT 0
  created_at      DATETIME
}
```

- [ ] Optional PIN protection
- [ ] Configurable max participants
- [ ] Optional recording of mixed audio

---

## 5. Flow Graph — Node Types

The call flow is a directed graph (with loops allowed for retries). Each node type maps to a processing function in the Go SIP engine.

| Node Type | Icon | Inputs | Outputs | Description |
|---|---|---|---|---|
| **Inbound Number** | 📞 | None (entry point) | 1 (next) | Entry node. One or more DIDs mapped to it. |
| **Time Switch** | 🕐 | 1 (incoming call) | N (one per rule + default) | Routes based on day/time rules. Each output edge labelled with the rule name. |
| **IVR Menu** | 🔢 | 1 (incoming call) | N (one per digit + timeout + invalid) | Plays prompt, collects DTMF. Each output edge labelled with digit. |
| **Ring Group** | 👥 | 1 (incoming call) | 2 (answered, no answer) | Rings a group of extensions per strategy. |
| **Extension** | 👤 | 1 (incoming call) | 2 (answered, no answer/busy) | Rings a single extension across all registered devices. |
| **Voicemail** | 📬 | 1 (incoming call) | 1 (after recording) | Records a voicemail into a specific voicemail box. |
| **Play Message** | 🔊 | 1 (incoming call) | 1 (after playback) | Plays an audio file or TTS, then continues. |
| **Conference** | 🎙️ | 1 (incoming call) | 1 (after leave) | Joins caller into a conference bridge. |
| **Transfer** | ↗️ | 1 (incoming call) | 0 (terminal) | Blind or attended transfer to external number or extension. |
| **Hangup** | ⛔ | 1 (incoming call) | 0 (terminal) | Terminates the call with configurable cause code. |
| **Set Caller ID** | 🏷️ | 1 (incoming call) | 1 (next) | Overrides caller ID name/number for downstream. |
| **Webhook** | 🌐 | 1 (incoming call) | N (conditional) | HTTP callout to external API, routes based on response. Future phase but design the node now. |
| **Queue** | ⏳ | 1 (incoming call) | 2 (answered, timeout) | ACD queue with hold music. Phase 2 but reserve the node type. |

### 5.1 Inline Entity Creation from Flow Editor

A core UX principle: **the flow editor is the primary interface for building a phone system.** Users should never have to leave the canvas to create the entities their flow needs.

- [ ] **Drag a new node onto the canvas → if no entity exists, prompt to create one inline**
  - [ ] Drag "Extension" node → modal/drawer to create a new extension (name, number, SIP creds) or select existing
  - [ ] Drag "Voicemail" node → modal/drawer to create a new voicemail box (name, greeting, email) or select existing
  - [ ] Drag "Ring Group" node → modal/drawer to create a new ring group (name, strategy, add members) or select existing
  - [ ] Drag "IVR Menu" node → modal/drawer to create a new IVR (name, greeting, digit mapping) or select existing
  - [ ] Drag "Time Switch" node → modal/drawer to create a new time switch (name, rules) or select existing
  - [ ] Drag "Conference" node → modal/drawer to create a new conference bridge (name, PIN) or select existing
  - [ ] Drag "Inbound Number" node → modal/drawer to create/assign a DID or select existing

- [ ] **Node config panel (click existing node) allows full CRUD**
  - [ ] Edit all properties of the linked entity directly in the side panel
  - [ ] "Create new" button within the entity selector dropdown
  - [ ] Changes save to both the entity and the flow graph
  - [ ] Delete entity from here (with confirmation + impact warning if used elsewhere)

- [ ] **Entity selector pattern**
  - [ ] Searchable dropdown for all entity-linked nodes
  - [ ] "New [Entity]..." option always at top of dropdown
  - [ ] Shows entity status (e.g., extension online/offline, trunk registered/failed)
  - [ ] Quick-edit pencil icon next to selected entity

- [ ] **Flow-first creation flow**
  - [ ] Example: user drags IVR node → clicks "New IVR Menu..." → fills in name + greeting → adds digit options → each digit option auto-creates output handles on the node → user connects edges to next nodes
  - [ ] Example: user drags Voicemail node → clicks "New Voicemail Box..." → fills in name + email → optionally links to extension for MWI → done
  - [ ] Example: user drags Ring Group node → clicks "New Ring Group..." → picks strategy → adds members from extension list (or creates new extensions inline) → done

### 5.2 Flow Graph JSON Structure

```json
{
  "nodes": [
    {
      "id": "node_1",
      "type": "inbound_number",
      "position": { "x": 100, "y": 200 },
      "data": {
        "label": "Main Number",
        "entity_id": 1,
        "entity_type": "inbound_number",
        "config": {}
      }
    },
    {
      "id": "node_2",
      "type": "time_switch",
      "position": { "x": 400, "y": 200 },
      "data": {
        "label": "Business Hours",
        "entity_id": 1,
        "entity_type": "time_switch",
        "config": {}
      }
    },
    {
      "id": "node_3",
      "type": "voicemail",
      "position": { "x": 700, "y": 400 },
      "data": {
        "label": "After Hours VM",
        "entity_id": 2,
        "entity_type": "voicemail_box",
        "config": {}
      }
    }
  ],
  "edges": [
    {
      "id": "edge_1-2",
      "source": "node_1",
      "target": "node_2",
      "sourceHandle": "next",
      "targetHandle": "incoming"
    },
    {
      "id": "edge_2-3",
      "source": "node_2",
      "target": "node_3",
      "sourceHandle": "default",
      "targetHandle": "incoming",
      "label": "After Hours"
    }
  ]
}
```

- [ ] Nodes reference entities via `entity_id` + `entity_type`
- [ ] Node position stored for canvas layout persistence
- [ ] Edge labels describe routing conditions
- [ ] Handles map to node-type-specific outputs (e.g., time switch has one handle per rule + default)

### 5.3 Flow Engine (Go)

The flow engine is a goroutine-per-call state machine:

- [ ] Incoming SIP INVITE arrives → match DID to `inbound_numbers` → find `flow_entry_node`
- [ ] Load published `call_flows` JSON
- [ ] Walk the graph: execute current node → follow output edge → next node
- [ ] Each node type has a handler: `func(ctx *CallContext, node Node) (outputEdge string, err error)`
- [ ] `CallContext` carries the SIP transaction, caller info, collected DTMF, variables
- [ ] CDR records the full traversal path
- [ ] Timeout handling per node
- [ ] Error handling: if node fails, try to gracefully terminate call

---

## 6. Admin UI

### 6.1 Pages / Views

| Page | Description |
|---|---|
| **Dashboard** | Active calls, registration count, recent CDRs, system health |
| **Call Flows** | React Flow canvas. List of flows, click to edit. Publish button. Primary interface. |
| **Trunks** | CRUD for SIP trunks. Registration status indicator. Test button. |
| **Inbound Numbers** | CRUD for DIDs. Map to flow entry node. |
| **Extensions** | CRUD for extensions/users. SIP credentials. Follow-me config. Registration status. |
| **Voicemail Boxes** | CRUD for voicemail boxes. Message browser. Play/download/delete. |
| **Ring Groups** | CRUD for ring groups. Member management. |
| **IVR Menus** | CRUD with audio upload / TTS preview. |
| **Time Switches** | Visual time grid editor + rule list. |
| **Conference Bridges** | CRUD. Show active participants. |
| **Recordings** | Browse/search call recordings. Play/download/delete. Storage usage. |
| **CDR / Call History** | Searchable/filterable call log. CSV export. |
| **Settings** | SIP ports, TLS certs, codecs, recording storage, SMTP for VM email, license key, push gateway config. |
| **Login** | Session-based admin auth. |

- [ ] All CRUD pages are secondary to the flow editor — entities can be fully managed from either location
- [ ] CRUD pages useful for bulk management, search, and overview
- [ ] Consistent design language across all pages

### 6.2 Admin Auth

- [ ] Session-based authentication (secure cookie + CSRF token)
- [ ] Initial setup wizard creates admin account on first boot
- [ ] Argon2id password hashing
- [ ] Optional TOTP 2FA (Phase 2, but design the DB field now)
- [ ] JWT tokens for API access from external integrations
- [ ] All API endpoints under `/api/v1/` require auth except `/api/v1/health`

### 6.3 Embedded SPA

- [ ] React app built with Vite
- [ ] Output placed in `internal/web/dist/`
- [ ] Embedded in Go binary via `//go:embed`
- [ ] chi serves at `/` with SPA fallback (all non-API routes → `index.html`)
- [ ] WebSocket endpoint `/ws` for real-time updates (active calls, registrations)

---

## 7. REST API Design

Base: `/api/v1`

### Core Resources

```
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
GET    /api/v1/auth/me

GET    /api/v1/dashboard/stats

# Trunks
GET    /api/v1/trunks
POST   /api/v1/trunks
GET    /api/v1/trunks/:id
PUT    /api/v1/trunks/:id
DELETE /api/v1/trunks/:id
POST   /api/v1/trunks/:id/test

# Inbound Numbers
GET    /api/v1/numbers
POST   /api/v1/numbers
GET    /api/v1/numbers/:id
PUT    /api/v1/numbers/:id
DELETE /api/v1/numbers/:id

# Extensions
GET    /api/v1/extensions
POST   /api/v1/extensions
GET    /api/v1/extensions/:id
PUT    /api/v1/extensions/:id
DELETE /api/v1/extensions/:id
GET    /api/v1/extensions/:id/registrations

# Voicemail Boxes
GET    /api/v1/voicemail-boxes
POST   /api/v1/voicemail-boxes
GET    /api/v1/voicemail-boxes/:id
PUT    /api/v1/voicemail-boxes/:id
DELETE /api/v1/voicemail-boxes/:id
GET    /api/v1/voicemail-boxes/:id/messages
DELETE /api/v1/voicemail-boxes/:id/messages/:msg_id
PUT    /api/v1/voicemail-boxes/:id/messages/:msg_id/read
GET    /api/v1/voicemail-boxes/:id/messages/:msg_id/audio
POST   /api/v1/voicemail-boxes/:id/greeting       -- upload custom greeting

# Ring Groups
GET    /api/v1/ring-groups
POST   /api/v1/ring-groups
GET    /api/v1/ring-groups/:id
PUT    /api/v1/ring-groups/:id
DELETE /api/v1/ring-groups/:id

# IVR Menus
GET    /api/v1/ivr-menus
POST   /api/v1/ivr-menus
GET    /api/v1/ivr-menus/:id
PUT    /api/v1/ivr-menus/:id
DELETE /api/v1/ivr-menus/:id

# Time Switches
GET    /api/v1/time-switches
POST   /api/v1/time-switches
GET    /api/v1/time-switches/:id
PUT    /api/v1/time-switches/:id
DELETE /api/v1/time-switches/:id

# Conference Bridges
GET    /api/v1/conferences
POST   /api/v1/conferences
GET    /api/v1/conferences/:id
PUT    /api/v1/conferences/:id
DELETE /api/v1/conferences/:id

# Call Flows
GET    /api/v1/flows
POST   /api/v1/flows
GET    /api/v1/flows/:id
PUT    /api/v1/flows/:id
DELETE /api/v1/flows/:id
POST   /api/v1/flows/:id/publish
POST   /api/v1/flows/:id/validate

# CDR
GET    /api/v1/cdrs
GET    /api/v1/cdrs/:id
GET    /api/v1/cdrs/export

# Recordings
GET    /api/v1/recordings
GET    /api/v1/recordings/:id/download
DELETE /api/v1/recordings/:id

# Audio Prompts
GET    /api/v1/prompts
POST   /api/v1/prompts                -- upload audio file
GET    /api/v1/prompts/:id/audio
DELETE /api/v1/prompts/:id

# System
GET    /api/v1/settings
PUT    /api/v1/settings
GET    /api/v1/health
GET    /api/v1/system/status               -- SIP stack status, trunk registrations
POST   /api/v1/system/reload               -- hot-reload config without restart

# Active Calls (WebSocket preferred, REST fallback)
GET    /api/v1/calls/active
POST   /api/v1/calls/:id/hangup
POST   /api/v1/calls/:id/transfer

# Mobile App Endpoints
POST   /api/v1/app/auth                    -- extension login (returns JWT)
GET    /api/v1/app/me                      -- extension profile
PUT    /api/v1/app/me                      -- update DND, follow-me etc
GET    /api/v1/app/voicemail               -- list voicemails for boxes linked to this extension
PUT    /api/v1/app/voicemail/:id/read      -- mark read
GET    /api/v1/app/voicemail/:id/audio     -- stream audio
GET    /api/v1/app/history                 -- call history for this extension
POST   /api/v1/app/push-token             -- register FCM/APNs token
```

- [ ] All CRUD endpoints return consistent JSON envelope `{ "data": ..., "error": ... }`
- [ ] Pagination via `?limit=N&offset=N` on list endpoints
- [ ] Filtering via query params (e.g., `?direction=inbound&from=2026-01-01`)
- [ ] WebSocket at `/ws` for real-time events (call state, registrations, trunk status)
- [ ] App endpoints use JWT auth (issued via `/api/v1/app/auth`)
- [ ] App voicemail endpoint returns messages from all voicemail boxes linked to the calling extension's `notify_extension_id`

---

## 8. SIP Engine Detail

### 8.1 Transports

- [ ] UDP on port 5060 (configurable)
- [ ] TCP on port 5060 (configurable)
- [ ] TLS on port 5061 (configurable, requires cert)
- [ ] WSS on port 8089 (for WebRTC/browser softphones — Phase 2 but configure the listener now)

### 8.2 Trunk Registration

- [ ] Outbound registration to upstream providers
- [ ] Periodic re-registration with configurable expiry
- [ ] Failover: if registration fails, retry with exponential backoff
- [ ] Health check: OPTIONS ping to detect trunk failure
- [ ] Multiple trunks with priority/weight for outbound routing

### 8.3 Inbound Call Handling

- [ ] INVITE received → match To/Request-URI to `inbound_numbers`
- [ ] If no match, check if it's an internal extension-to-extension call
- [ ] If matched to flow → spawn call handler goroutine → walk the graph
- [ ] Send `100 Trying` immediately
- [ ] Each node executes and determines next step
- [ ] SDP negotiation: always use media proxy for consistent NAT handling

### 8.4 Outbound Call Handling

- [ ] Extension dials a number
- [ ] Match against outbound routes (prefix matching, trunk selection by priority)
- [ ] Apply caller ID rules (extension CID, trunk CID, or override)
- [ ] Send INVITE to selected trunk
- [ ] Bridge the call

### 8.5 Media Proxy (RTP Engine)

- [ ] Always-on RTP relay (simplifies NAT, recording, conferencing)
- [ ] Port range: configurable (default 10000–20000)
- [ ] G.711 alaw (PCMA, payload 8)
- [ ] G.711 ulaw (PCMU, payload 0)
- [ ] Opus (payload 111, 48kHz)
- [ ] Transcoding between endpoints with different codec support
- [ ] DTMF: RFC 2833 (telephone-event) and SIP INFO fallback
- [ ] Call recording: fork media stream to WAV writer when enabled
- [ ] Conference mixing: N-way audio mixing in the RTP engine
- [ ] Session timeout / cleanup for orphaned streams
- [ ] NAT handling: symmetric RTP, learn remote port from first packet

### 8.6 Voicemail

- [ ] Triggered by voicemail flow node — routes to a specific voicemail box
- [ ] Plays greeting from the target voicemail box (custom upload or default)
- [ ] Records to WAV file (G.711) with configurable max duration (per-box setting)
- [ ] Stores metadata in `voicemail_messages` linked to the voicemail box
- [ ] MWI (Message Waiting Indicator) via SIP NOTIFY to the extension linked via `notify_extension_id`
- [ ] Optional email notification with audio attachment (SMTP config, per-box setting)
- [ ] Direct-to-voicemail: a DID can flow straight to a voicemail node with no ringing

### 8.7 Follow-Me

- [ ] Per-extension configuration
- [ ] Sequential ring: desk phone first → after X seconds → mobile number
- [ ] Simultaneous ring: ring all devices at once
- [ ] External numbers dialled via outbound trunk
- [ ] Configurable per-destination ring timeout
- [ ] Confirmation prompt for external legs ("Press 1 to accept this call") to prevent voicemail pickup

---

## 9. Flutter Softphone App

### 9.1 Core Features

- [ ] SIP registration to PBX over TLS/TCP
- [ ] Make and receive calls (G.711, Opus)
- [ ] DTMF sending
- [ ] Call history (synced from PBX API)
- [ ] Voicemail list + playback (streamed from PBX API, shows all boxes linked to this extension)
- [ ] Push notification wake-up for incoming calls
- [ ] DND toggle
- [ ] Follow-me toggle
- [ ] Contact directory (from PBX)
- [ ] Call transfer (blind)

### 9.2 Platform Requirements

- [ ] iOS 15+ (CallKit integration for native call UI)
- [ ] Android 10+ (ConnectionService for native call integration)
- [ ] Background audio session handling
- [ ] Works on WiFi, 4G, 5G — SRTP for encryption over untrusted networks
- [ ] Battery optimization whitelisting guidance in-app

### 9.3 SIP Library

- [ ] Evaluate `dart_sip_ua` / `flutter_ooh_sip` / `ooh_ooh` native bridge
- [ ] Must support: TLS transport, SRTP (SDES), Opus codec, push wake-up
- [ ] Fallback: native iOS (Swift) and Android (Kotlin) SIP engines with Flutter UI on top via platform channels

### 9.4 Authentication

- [ ] User enters: PBX server URL + extension number + SIP password
- [ ] App calls `/api/v1/app/auth` → receives JWT + SIP config
- [ ] JWT used for all REST API calls
- [ ] SIP credentials used for SIP registration
- [ ] Push token sent to PBX via `/api/v1/app/push-token`
- [ ] PBX stores token in `registrations` table

---

## 10. Push Gateway & License Server

### 10.1 Purpose

- [ ] Centrally hosted by The IT Dept
- [ ] Receives push requests from FlowPBX instances
- [ ] Delivers push notifications via FCM (Android) and APNs (iOS)
- [ ] Validates license keys (PBX instance sends license key with each push request)
- [ ] Tracks active installations, extension counts

### 10.2 Architecture

- [ ] Separate binary in same repo (`cmd/pushgw`), shares Go module
- [ ] PostgreSQL for license management (multi-tenant)
- [ ] Stateless — can be horizontally scaled
- [ ] `POST /v1/push` — send push notification (requires license key header)
- [ ] `POST /v1/license/validate` — validate license key, return entitlements
- [ ] `POST /v1/license/activate` — activate a new installation
- [ ] `GET  /v1/license/status` — check license status

### 10.3 Push Flow

- [ ] Incoming call to PBX → extension not registered (app backgrounded)
- [ ] PBX checks `registrations` for push token
- [ ] PBX sends push request to Push Gateway: `{license_key, push_token, push_platform, caller_id, call_id}`
- [ ] Push Gateway validates license → sends push via FCM/APNs
- [ ] Phone wakes up → app connects to PBX via SIP → PBX rings the extension
- [ ] Timeout: if app doesn't register within 5 seconds, continue flow (voicemail etc)

### 10.4 License Tiers (design for, implement later)

- [ ] Free: up to 5 extensions, community support
- [ ] Standard: up to 25 extensions
- [ ] Professional: up to 100 extensions
- [ ] Fields in DB: `license_key`, `tier`, `max_extensions`, `expires_at`, `instance_id`, `activated_at`

---

## 11. Configuration & First Boot

### 11.1 Binary Startup
```bash
./flowpbx                          # defaults: UDP/TCP 5060, HTTP 8080, data in ./data/
./flowpbx --sip-port 5060 --http-port 443 --tls-cert cert.pem --tls-key key.pem --data-dir /var/lib/flowpbx
```

### 11.2 First Boot Wizard

- [ ] Binary starts → detects empty database
- [ ] Admin opens browser → redirected to `/setup`
- [ ] Wizard step: Set admin username + password
- [ ] Wizard step: Set PBX hostname / external IP
- [ ] Wizard step: Configure SIP ports
- [ ] Wizard step: (Optional) Add first trunk
- [ ] Wizard step: (Optional) Add first extension
- [ ] Wizard step: Enter license key (or skip for free tier)
- [ ] Wizard writes config → redirects to login

### 11.3 Environment Variables

All config can be set via env vars with `FLOWPBX_` prefix:
```
FLOWPBX_HTTP_PORT=8080
FLOWPBX_SIP_UDP_PORT=5060
FLOWPBX_SIP_TCP_PORT=5060
FLOWPBX_SIP_TLS_PORT=5061
FLOWPBX_DATA_DIR=./data
FLOWPBX_EXTERNAL_IP=203.0.113.1
FLOWPBX_LOG_LEVEL=info
FLOWPBX_LICENSE_KEY=xxx
FLOWPBX_PUSH_GATEWAY_URL=https://push.flowpbx.io
```

- [ ] All settings available as env vars and CLI flags
- [ ] CLI flags take precedence over env vars
- [ ] Env vars take precedence over database config
- [ ] Document all variables in README

---

## 12. Deployment & Build

### 12.1 Build Pipeline (GitHub Actions)

- [ ] Trigger: push to main (CI), tags for releases
- [ ] Lint Go (golangci-lint)
- [ ] Test Go (go test ./...)
- [ ] Build React UI (npm run build)
- [ ] Embed UI into Go binary
- [ ] Cross-compile: linux/amd64, linux/arm64
- [ ] Create GitHub Release with binaries
- [ ] (Future) Build Docker image

### 12.2 Target Specs

- [ ] Minimum: 1 vCPU, 512MB RAM, 10GB disk
- [ ] Recommended: 2 vCPU, 1GB RAM, 50GB disk (for recordings)
- [ ] Network: public IP or port-forwarded UDP 5060 + RTP range

### 12.3 File System Layout
```
/var/lib/flowpbx/              # --data-dir
├── flowpbx.db                 # SQLite database
├── recordings/                # Call recordings
│   └── 2026/01/15/            # Date-organized
│       └── call_xxx.wav
├── voicemail/                 # Voicemail messages
│   └── box_1/                 # Organized by voicemail box ID
│       └── msg_xxx.wav
├── prompts/                   # IVR prompts, greetings
│   ├── system/                # Default prompts (extracted from binary on first boot)
│   └── custom/                # User-uploaded prompts
└── greetings/                 # Voicemail box greetings
    └── box_1.wav
```

- [ ] All paths relative to `--data-dir`
- [ ] Voicemail organized by box ID (not extension)
- [ ] Recordings organized by date
- [ ] Custom prompts separated from system prompts

---

## 13. Phased Delivery Plan

### Phase 1A — Foundation (Weeks 1–3)

**Goal:** Go binary boots, serves admin UI, SIP stack listens and responds.

- [ ] **Project scaffolding**
  - [ ] Initialize Go module (`github.com/flowpbx/flowpbx`)
  - [ ] Set up repo structure: `cmd/`, `internal/`, `pkg/`, `web/`, `migrations/`
  - [ ] GitHub Actions: lint + test + build pipeline
  - [ ] Cross-compilation targets: linux/amd64, linux/arm64
  - [ ] Makefile with targets: `build`, `dev`, `test`, `lint`, `ui-build`, `release`

- [ ] **Database layer**
  - [ ] SQLite integration with WAL mode
  - [ ] Migration system (embed SQL files, run on startup)
  - [ ] Initial schema: `system_config`, `extensions`, `trunks`, `inbound_numbers`, `voicemail_boxes`, `registrations`
  - [ ] Repository pattern: interfaces for all data access
  - [ ] Encrypted field support for passwords (AES-256-GCM, key from config)

- [ ] **HTTP server**
  - [ ] chi router setup with middleware stack (logging, recovery, CORS, auth)
  - [ ] Session-based admin auth (Argon2id hashing, secure cookies)
  - [ ] First-boot detection + setup wizard API
  - [ ] Health check endpoint
  - [ ] Static file serving via `//go:embed`

- [ ] **Admin UI shell**
  - [ ] Vite + React 18 + Tailwind CSS project in `web/`
  - [ ] Router setup (React Router)
  - [ ] Login page
  - [ ] Setup wizard UI
  - [ ] Layout: sidebar nav, header, content area
  - [ ] Dashboard page (placeholder stats)
  - [ ] Extensions CRUD pages
  - [ ] Trunks CRUD pages
  - [ ] Voicemail Boxes CRUD pages

- [ ] **SIP stack initialization**
  - [ ] sipgo UA + Server setup
  - [ ] UDP + TCP listeners on configurable ports
  - [ ] SIP REGISTER handler: authenticate against `extensions` table, store in `registrations`
  - [ ] Registration expiry cleanup goroutine
  - [ ] SIP OPTIONS responder (for health checks from trunks/phones)
  - [ ] Logging: structured SIP message logging (configurable verbosity)

### Phase 1B — Call Handling (Weeks 4–6)

**Goal:** Extension-to-extension calls work. Trunks register. Inbound calls land.

- [ ] **Outbound trunk registration**
  - [ ] Registration client for register-type trunks
  - [ ] Re-registration timer with exponential backoff on failure
  - [ ] Trunk status tracking (registered/failed/disabled)
  - [ ] IP-auth trunk support (ACL-based, no registration)
  - [ ] Admin UI: trunk status indicators (green/red)

- [ ] **RTP media proxy**
  - [ ] UDP socket pool for RTP relay (configurable port range)
  - [ ] SDP parsing and rewriting (replace endpoint IPs with proxy)
  - [ ] G.711 alaw/ulaw passthrough
  - [ ] Opus passthrough
  - [ ] DTMF relay (RFC 2833 telephone-event)
  - [ ] Session timeout / cleanup for orphaned streams
  - [ ] NAT handling: symmetric RTP, learn remote port from first packet

- [ ] **Internal calls (extension to extension)**
  - [ ] INVITE handler: look up target extension → find registrations → ring
  - [ ] Multi-device ringing (INVITE forked to all registered contacts)
  - [ ] Early media / 180 Ringing / 183 Session Progress
  - [ ] BYE handling and CDR creation
  - [ ] CANCEL handling (caller hangs up before answer)
  - [ ] Busy detection (486 Busy Here)

- [ ] **Inbound calls via trunk**
  - [ ] Match incoming INVITE to DID in `inbound_numbers`
  - [ ] Route to destination extension (direct, before flow engine exists)
  - [ ] Caller ID passthrough from trunk
  - [ ] CDR recording for all inbound calls

- [ ] **Outbound calls via trunk**
  - [ ] Outbound dialling from extensions
  - [ ] Trunk selection (ordered by priority)
  - [ ] Prefix manipulation (strip/add)
  - [ ] Caller ID application (extension, trunk, or override)
  - [ ] CDR recording for all outbound calls

- [ ] **CDR system**
  - [ ] CDR creation on call start, updated on answer and hangup
  - [ ] Hangup cause mapping (SIP response codes → friendly labels)
  - [ ] Admin UI: CDR list page with search/filter/date range
  - [ ] CSV export

### Phase 1C — Flow Engine & Nodes (Weeks 7–10)

**Goal:** Visual call flow editor works. Calls route through the flow graph. Entities can be created inline from the canvas.

- [ ] **Flow engine core**
  - [ ] Graph walker: load published flow → resolve entry node → execute → follow edges
  - [ ] CallContext struct: SIP transaction, caller info, variables, DTMF buffer, traversal path
  - [ ] Node handler interface: `Execute(ctx *CallContext) (outputEdge string, err error)`
  - [ ] Node-to-entity resolution: load entity by `entity_id` + `entity_type` from node data
  - [ ] Timeout handling per node
  - [ ] Error handling: if node fails, try to gracefully terminate call
  - [ ] Flow path recording in CDR (`flow_path` field)

- [ ] **Flow node implementations**
  - [ ] Inbound Number node (entry point, DID matching)
  - [ ] Extension node (ring with timeout, output: answered/no-answer)
  - [ ] Ring Group node (ring_all strategy first, then round_robin, random)
  - [ ] Time Switch node (evaluate rules against current time, follow matching edge)
  - [ ] IVR Menu node (play prompt, collect DTMF, route by digit)
  - [ ] Voicemail node (record into target voicemail box, play box greeting, store, trigger MWI)
  - [ ] Play Message node (play audio file, continue)
  - [ ] Hangup node (terminate call with cause)
  - [ ] Set Caller ID node (modify caller ID for downstream)
  - [ ] Transfer node (blind transfer to number or extension)
  - [ ] Conference node (join bridge)

- [ ] **Audio prompt system**
  - [ ] Default system prompts embedded in binary (WAV, G.711)
  - [ ] Extract to filesystem on first boot
  - [ ] Custom prompt upload via admin API + UI
  - [ ] Audio format validation and conversion (ffmpeg or native Go)
  - [ ] Prompt playback via RTP: read WAV → packetize → send

- [ ] **IVR DTMF collection**
  - [ ] Collect digits from RFC 2833 events
  - [ ] Inter-digit timeout handling
  - [ ] Max digits / terminator digit (#)
  - [ ] Buffer management per-call

- [ ] **Voicemail system**
  - [ ] Record incoming RTP to WAV file
  - [ ] Configurable max recording duration (per voicemail box)
  - [ ] Custom greeting per voicemail box (upload via API/UI)
  - [ ] Default greeting fallback
  - [ ] Voicemail storage and metadata in `voicemail_messages`
  - [ ] MWI: send SIP NOTIFY to extension linked via `notify_extension_id` on new message
  - [ ] Email notification (SMTP integration) with WAV attachment (per-box setting)
  - [ ] Auto-delete messages older than retention_days (per-box setting)
  - [ ] Admin UI: voicemail browser per box
  - [ ] API: voicemail box CRUD, message list, playback stream, delete, mark read

- [ ] **React Flow editor**
  - [ ] React Flow canvas component
  - [ ] Custom node components for each node type (styled, with config panels)
  - [ ] Edge handling with labels (e.g., "Digit 1", "Business Hours", "No Answer")
  - [ ] Node config side panel: click node → edit settings in a drawer/panel
  - [ ] Add node: drag from palette or right-click menu
  - [ ] Inline entity creation: "New [Entity]..." option when placing/configuring nodes
  - [ ] Inline entity editing: full CRUD for linked entity in side panel
  - [ ] Entity selector dropdown with search, status indicators, and "Create new" option
  - [ ] Save flow (auto-save draft)
  - [ ] Publish flow (snapshot current → mark as published)
  - [ ] Flow validation: check for disconnected nodes, missing configs, orphan paths
  - [ ] Visual feedback: highlight invalid nodes/edges in red
  - [ ] Multiple flows support (one per inbound number group, or reusable)

- [ ] **Time switch UI**
  - [ ] Rule editor: day checkboxes + time range pickers
  - [ ] Visual weekly grid preview
  - [ ] Holiday / specific date overrides
  - [ ] Timezone selector

- [ ] **IVR menu UI**
  - [ ] Digit mapping editor (0-9, *, #)
  - [ ] Audio prompt upload / select from library
  - [ ] Timeout and invalid handling config

### Phase 1D — Conference, Recording & Follow-Me (Weeks 11–13)

**Goal:** Conference bridges work. Call recording works. Follow-me works.

- [ ] **Conference bridge**
  - [ ] Audio mixing engine: N-way mixing in RTP proxy
  - [ ] Conference room management: create/join/leave/kick
  - [ ] PIN-protected entry
  - [ ] Mute/unmute participants
  - [ ] Admin UI: conference management, view active participants
  - [ ] Conference recording (mixed output to WAV)

- [ ] **Call recording**
  - [ ] Per-extension recording config (always / never / on-demand)
  - [ ] Per-trunk recording config
  - [ ] Global recording policy
  - [ ] Fork RTP stream to WAV writer (separate goroutine, non-blocking)
  - [ ] Date-organized file storage
  - [ ] Recording metadata in CDR
  - [ ] Admin UI: recording browser with playback, download, delete
  - [ ] Storage usage monitoring and alerts
  - [ ] Retention policy: auto-delete recordings older than X days (configurable)

- [ ] **Follow-me**
  - [ ] Sequential ring: ring registered devices → after timeout → ring external numbers
  - [ ] Simultaneous ring option
  - [ ] External number dialing via outbound trunk
  - [ ] Confirmation prompt on external legs ("Press 1 to accept")
  - [ ] Admin UI: follow-me config per extension
  - [ ] App API: toggle follow-me on/off

### Phase 1E — Mobile App & Push Gateway (Weeks 14–18)

**Goal:** Flutter app makes/receives calls. Push notifications wake the app.

- [ ] **Push Gateway service**
  - [ ] New Go project / repo
  - [ ] PostgreSQL schema: licenses, installations, push_logs
  - [ ] FCM integration (Firebase Admin SDK for Go)
  - [ ] APNs integration (HTTP/2 provider API)
  - [ ] Push endpoint: validate license → send push → log result
  - [ ] License validation endpoint
  - [ ] License activation endpoint (generates instance ID)
  - [ ] Rate limiting per license key
  - [ ] Health monitoring and logging
  - [ ] Deploy: containerized, hosted by TITD
  - [ ] Admin dashboard for license management (simple, internal)

- [ ] **PBX ↔ Push Gateway integration**
  - [ ] On incoming call to offline extension: check for push token in registrations
  - [ ] Send push request to gateway with call metadata
  - [ ] Wait for registration (configurable timeout, default 5s)
  - [ ] If no registration within timeout: continue flow (voicemail etc)
  - [ ] Push token management: store/update/invalidate

- [ ] **Flutter softphone app**
  - [ ] Project setup: Flutter, state management (Riverpod or Bloc)
  - [ ] Login screen: server URL + extension + password
  - [ ] SIP library integration (evaluate and select)
  - [ ] SIP registration over TLS/TCP
  - [ ] Outbound calls: dialpad, contact search
  - [ ] Inbound calls: full-screen incoming call UI
  - [ ] iOS: CallKit integration (native call UI, lock screen answering)
  - [ ] Android: ConnectionService integration
  - [ ] In-call screen: mute, speaker, hold, DTMF pad, transfer, hang up
  - [ ] Call history (from PBX API, cached locally)
  - [ ] Voicemail list + playback (all boxes linked to this extension, stream from PBX API)
  - [ ] DND toggle (updates PBX via API)
  - [ ] Follow-me toggle
  - [ ] Push notification handling:
    - [ ] FCM setup (Android)
    - [ ] APNs/PushKit setup (iOS — VoIP push for call wake-up)
    - [ ] On push received: wake SIP stack → register → receive INVITE
  - [ ] SRTP support for encrypted media
  - [ ] Codec support: G.711, Opus
  - [ ] Background mode handling (iOS/Android audio session management)
  - [ ] Battery optimization guidance (in-app prompt)
  - [ ] App icon and branding

### Phase 1F — Polish & Hardening (Weeks 19–21)

**Goal:** Production-ready. Stable. Documented.

- [ ] **Testing**
  - [ ] Go unit tests for all core packages (target 70%+ coverage)
  - [ ] SIP integration tests (use sipp for automated call testing)
  - [ ] Flow engine tests: validate each node type with mock calls
  - [ ] API endpoint tests
  - [ ] UI component tests (React Testing Library)
  - [ ] Flutter widget and integration tests
  - [ ] Load testing: 50 concurrent calls on minimum spec hardware
  - [ ] Push testing: app backgrounded → push → wake → answer (both platforms)
  - [ ] Trunk failover testing
  - [ ] Voicemail box tests: recording, MWI, email notification, retention cleanup

- [ ] **Security hardening**
  - [ ] SIP auth: nonce replay prevention, brute-force lockout
  - [ ] Fail2ban-style IP blocking for failed SIP auth attempts
  - [ ] Rate limiting on all API endpoints
  - [ ] HTTPS enforcement for admin UI (auto Let's Encrypt or manual cert)
  - [ ] Secrets encryption at rest (SIP passwords, trunk credentials)
  - [ ] Input validation on all API endpoints
  - [ ] SQL injection prevention (parameterized queries only)
  - [ ] CSRF protection on admin UI
  - [ ] Security headers (CSP, HSTS, etc.)

- [ ] **Monitoring & observability**
  - [ ] Structured JSON logging (configurable level)
  - [ ] SIP message logging (pcap-style, configurable)
  - [ ] Prometheus metrics endpoint `/metrics` (optional)
    - [ ] Active calls gauge
    - [ ] Registered extensions gauge
    - [ ] Trunk status gauge
    - [ ] Call volume counter (inbound/outbound/internal)
    - [ ] RTP packet loss / jitter metrics
    - [ ] Voicemail box message counts
  - [ ] Admin dashboard: real-time stats via WebSocket

- [ ] **Documentation**
  - [ ] README with quickstart
  - [ ] Admin guide: installation, configuration, trunk setup
  - [ ] User guide: mobile app setup, voicemail, follow-me
  - [ ] API documentation (OpenAPI / Swagger)
  - [ ] Troubleshooting guide (common SIP issues)
  - [ ] Architecture documentation for developers

- [ ] **Operational**
  - [ ] Graceful shutdown (drain active calls, de-register trunks)
  - [ ] Config hot-reload (reload flows, extensions without restart)
  - [ ] Database backup guidance (SQLite .backup command)
  - [ ] Log rotation
  - [ ] Automatic cleanup of old recordings (configurable retention)
  - [ ] Automatic cleanup of old voicemail messages (per-box retention_days)
  - [ ] Version check against push gateway (notify admin of updates)
  - [ ] Startup self-test: verify ports available, external IP reachable, DNS resolution

---

## 14. Non-Functional Requirements

- [ ] Binary size: < 50MB (including embedded UI and prompts)
- [ ] Memory usage (idle): < 64MB
- [ ] Memory per active call: < 2MB
- [ ] Call setup time: < 200ms (internal), < 500ms (via trunk)
- [ ] Max concurrent calls: 50+ on minimum spec
- [ ] Database size (no recordings): < 100MB for 100 extensions + 1M CDRs
- [ ] Admin UI load time: < 2 seconds
- [ ] Startup time: < 3 seconds to SIP-ready
- [ ] Hot reload: < 500ms for config changes
- [ ] Supported OS: Linux amd64, Linux arm64
- [ ] Go version: 1.22+
- [ ] Node.js (build only): 20 LTS
- [ ] Flutter: 3.x stable

---

## 15. Risks & Mitigations

- [ ] **sipgo lacks features we need (e.g., proper dialog management)** — HIGH — Fork and extend. sipgo is MIT licensed. Evaluate early in Phase 1A.
- [ ] **RTP media handling complexity (transcoding, mixing)** — HIGH — Start with passthrough only. Add transcoding via Opus C bindings or pure Go. Conference mixing is the hardest — prototype early.
- [ ] **Flutter SIP library maturity** — MEDIUM — Evaluate multiple options in first week. Fallback: native SIP engines with Flutter UI via platform channels.
- [ ] **NAT traversal edge cases** — MEDIUM — Always-on media proxy eliminates most NAT issues. STUN/ICE not needed if proxy handles all RTP.
- [ ] **SQLite concurrency under load** — LOW — WAL mode handles concurrent reads well. Single-writer is fine for config. CDR writes are sequential per-call.
- [ ] **iOS push notification reliability** — MEDIUM — VoIP pushes via PushKit are high priority. Test extensively. Fallback: regular push + local notification.
- [ ] **Codec licensing (G.729)** — NONE — Not using G.729. G.711 is royalty-free. Opus is royalty-free.

---

## 16. Go Project Structure

```
flowpbx/
├── cmd/
│   ├── flowpbx/
│   │   └── main.go                    # PBX entry point
│   └── pushgw/
│       └── main.go                    # Push gateway / license server entry point
├── internal/
│   ├── config/                        # Config loading, env vars, CLI flags
│   ├── database/                      # SQLite connection, migrations
│   │   ├── migrations/                # Embedded SQL migration files
│   │   └── models/                    # Generated or hand-written models
│   ├── api/                           # HTTP handlers
│   │   ├── middleware/                # Auth, logging, CORS, rate limit
│   │   ├── admin/                     # Admin API handlers
│   │   ├── app/                       # Mobile app API handlers
│   │   └── ws/                        # WebSocket handlers
│   ├── sip/                           # SIP engine
│   │   ├── server.go                  # sipgo setup, listeners
│   │   ├── registrar.go               # REGISTER handler
│   │   ├── invite.go                  # INVITE handler, call setup
│   │   ├── dialog.go                  # Dialog/session management
│   │   ├── trunk.go                   # Trunk registration, health
│   │   └── auth.go                    # SIP digest auth
│   ├── media/                         # RTP/media handling
│   │   ├── proxy.go                   # RTP relay/proxy
│   │   ├── mixer.go                   # Conference audio mixing
│   │   ├── recorder.go               # Call recording
│   │   ├── player.go                  # Audio prompt playback
│   │   ├── codecs/                    # Codec implementations
│   │   └── dtmf.go                    # DTMF detection/generation
│   ├── flow/                          # Call flow engine
│   │   ├── engine.go                  # Graph walker
│   │   ├── context.go                 # CallContext
│   │   ├── validator.go               # Flow validation
│   │   └── nodes/                     # Node type handlers
│   │       ├── inbound.go
│   │       ├── extension.go
│   │       ├── ringgroup.go
│   │       ├── timeswitch.go
│   │       ├── ivr.go
│   │       ├── voicemail.go
│   │       ├── playmessage.go
│   │       ├── conference.go
│   │       ├── transfer.go
│   │       ├── hangup.go
│   │       ├── setcallerid.go
│   │       └── webhook.go            # Stub for future
│   ├── voicemail/                     # Voicemail box + message management
│   ├── recording/                     # Recording management
│   ├── push/                          # Push gateway client
│   └── license/                       # License validation
├── web/                               # React admin UI
│   ├── src/
│   │   ├── components/
│   │   │   ├── flow/                  # React Flow node components + inline entity forms
│   │   │   ├── entities/              # Shared entity CRUD forms (used in flow + pages)
│   │   │   ├── layout/                # Sidebar, header, etc.
│   │   │   └── common/                # Shared UI components
│   │   ├── pages/                     # Route pages
│   │   ├── api/                       # API client
│   │   ├── hooks/                     # Custom React hooks
│   │   └── store/                     # State management
│   ├── package.json
│   ├── tailwind.config.js
│   └── vite.config.js
├── migrations/                        # SQL migration files
├── prompts/                           # Default audio prompts (embedded)
├── .github/
│   └── workflows/
│       ├── ci.yml                     # Lint + test on PR
│       └── release.yml                # Build + release on tag
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

- [ ] Entity forms in `web/src/components/entities/` are shared between CRUD pages and flow editor inline creation
- [ ] Flow node components in `web/src/components/flow/` import entity forms for inline CRUD
- [ ] This ensures consistent UI whether creating entities from the canvas or from dedicated pages

---

## 17. Open Decisions

- [ ] **Product name:** FlowPBX / FlowBox / SwitchFlow / CallGraph / ? — needs to be available as a domain
- [ ] **SQLite driver:** `mattn/go-sqlite3` (CGO) vs `modernc.org/sqlite` (pure Go) — CGO gives better perf but complicates cross-compile. Pure Go is simpler. Benchmark.
- [ ] **Flutter SIP library:** `dart_sip_ua` / native bridge / custom — spike in Phase 1E week 1
- [ ] **Opus in Go:** `hraban/opus` (CGO, libopus) vs pure Go — CGO likely needed for real-time transcoding performance
- [ ] **Audio format for prompts:** WAV (G.711) embedded vs convert on playback — pre-convert to G.711 WAV for zero-latency playback
- [ ] **WebRTC support:** Phase 2 or never? — WSS listener + SRTP + ICE = significant scope. Park it.
- [ ] **Multi-tenant (future):** Separate product or mode? — design data model to not preclude it, but don't build it
- [ ] **TTS engine for IVR:** None / Google TTS API / local (piper) — nice to have, don't block on it

---

## 18. Success Criteria — Phase 1 Complete

- [ ] Single binary runs on a fresh Linux VPS with no dependencies
- [ ] Admin can log in, create trunks, extensions, voicemail boxes, and build a call flow visually
- [ ] Entities (extensions, ring groups, voicemail boxes, IVRs, time switches) can be created inline from the flow editor
- [ ] Inbound call arrives via trunk → traverses flow → rings extension → answered → audio works → CDR recorded
- [ ] Outbound call from extension → routes via trunk → audio works → CDR recorded
- [ ] Extension-to-extension calls work with audio
- [ ] Time switch correctly routes calls based on time of day
- [ ] IVR plays prompt, collects digit, routes to correct destination
- [ ] Voicemail box records message, stores, sends MWI to linked extension, sends email notification
- [ ] DID can route directly to a voicemail box (no ringing)
- [ ] Conference bridge handles 3+ participants with mixed audio
- [ ] Call recording captures both sides of the conversation
- [ ] Flutter app registers, makes calls, receives calls (including via push when backgrounded)
- [ ] App can view and play voicemail messages from linked voicemail boxes
- [ ] Follow-me rings external number after desk phone timeout
- [ ] Push gateway delivers notifications reliably on iOS and Android
- [ ] System handles 50 concurrent calls on 2 vCPU / 1GB RAM without degradation