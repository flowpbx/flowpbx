# Planning Mode - FlowPBX

You are planning the implementation of FlowPBX - a single-binary, self-hosted PBX system with a visual call flow editor.

## Your Task

Review sprints, perform gap analysis, and update sprint tasks. Do NOT implement anything.

## Sprint System

Sprints are in `.ralph/sprints/sprint-XX.md` (01-20).

Each sprint contains:
- Duration and focus area
- Task checklist with `- [ ]` / `- [x]` markers
- Referenced specs to read
- Dependencies on other sprints

## Project Context

**Target Users**: Small-to-medium businesses (5-100 extensions) replacing legacy PBX platforms.

**Core Thesis**: If you can draw a flowchart, you can build a phone system.

**Tech Stack**:
- **Backend**: Go 1.22+, single binary, chi router, SQLite (WAL mode)
- **SIP**: sipgo (github.com/emiago/sipgo)
- **Admin UI**: React 18 + Tailwind CSS + React Flow, embedded via `//go:embed`
- **Mobile App**: Flutter (iOS + Android)
- **Push Gateway**: Separate Go service, FCM + APNs, multi-tenant

**Architecture**: Single Go binary containing SIP stack, RTP media proxy, HTTP server (admin API + embedded React SPA), SQLite database. Call routing logic built visually using React Flow drag-and-drop graph editor. Companion Flutter softphone app. Centrally-hosted push gateway doubles as license server.

**Core Features**:
- SIP trunking (register + IP auth)
- Extension management with multi-device registration
- Visual call flow editor (React Flow)
- Flow nodes: Inbound Number, Time Switch, IVR Menu, Ring Group, Extension, Voicemail, Play Message, Conference, Transfer, Hangup, Set Caller ID, Webhook (future), Queue (future)
- Inline entity creation from the flow canvas
- RTP media proxy with G.711 + Opus, DTMF, recording, conference mixing
- Voicemail boxes (independent entities, not tied to extensions)
- CDR and call recording
- Follow-me (sequential/simultaneous ring to external numbers)
- Flutter softphone with push wake-up
- License server + push gateway (separate service)

## PRD Reference

The full PRD is at `.ralph/flowpbx.md`. Always read it for detailed specs on:
- Data model (Section 4)
- Flow graph node types (Section 5)
- Admin UI pages (Section 6)
- REST API design (Section 7)
- SIP engine detail (Section 8)
- Flutter app (Section 9)
- Push gateway (Section 10)
- Configuration (Section 11)
- Project structure (Section 16)

## Instructions

1. **Read** sprint files in `.ralph/sprints/`
2. **Read** the PRD at `.ralph/flowpbx.md` for referenced sections
3. **Analyze** current source code
4. **Identify** gaps between PRD and sprint tasks
5. **Update** sprint files with missing tasks or clarifications
6. **Commit** changes with `docs(sprint): description`

## Rules

- Do NOT implement anything
- Only update sprint task lists
- Ensure all PRD requirements are covered in tasks
- Keep tasks atomic and testable
- Respect sprint dependencies (earlier sprints provide foundation for later ones)

## Completion

When done reviewing all sprints, output `<promise>DONE</promise>`
