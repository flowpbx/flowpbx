# Sprint 15 — React Flow Editor

**Phase**: 1C (Flow Engine & Nodes)
**Focus**: Visual call flow editor with React Flow, inline entity creation, publish/validate
**Dependencies**: Sprint 04, Sprint 11

**PRD Reference**: Section 5 (Flow Graph Node Types), Section 5.1 (Inline Entity Creation), Section 5.2 (Flow Graph JSON), Section 6.1 (Call Flows page)

## Tasks

- [x] Install React Flow dependency in web project
- [x] Create React Flow canvas component with drag-and-drop support
- [x] Create node palette/toolbar — draggable node types for all flow nodes
- [x] Create custom node component: Inbound Number (entry point, single output handle)
- [x] Create custom node component: Time Switch (single input, N output handles per rule + default)
- [x] Create custom node component: IVR Menu (single input, N output handles per digit + timeout + invalid)
- [x] Create custom node component: Ring Group (single input, 2 outputs: answered / no answer)
- [x] Create custom node component: Extension (single input, 2 outputs: answered / no answer)
- [x] Create custom node component: Voicemail (single input, 1 output: after recording)
- [x] Create custom node component: Play Message (single input, 1 output: after playback)
- [x] Create custom node component: Conference (single input, 1 output: after leave)
- [x] Create custom node component: Transfer (single input, terminal)
- [x] Create custom node component: Hangup (single input, terminal)
- [x] Create custom node component: Set Caller ID (single input, 1 output)
- [x] Create edge components with labels (e.g., "Digit 1", "Business Hours", "No Answer")
- [x] Create node config side panel: click node → drawer/panel with entity settings
- [x] Implement inline entity creation: drag new node → "New [Entity]..." option → modal/drawer to create
- [x] Implement inline entity editing: full CRUD for linked entity in side panel
- [x] Create entity selector dropdown with search, status indicators, "Create new" option
- [x] Share entity form components between flow editor and CRUD pages (`web/src/components/entities/`)
- [x] Implement flow save (auto-save draft to API)
- [x] Implement flow publish (call `POST /api/v1/flows/:id/publish`)
- [x] Implement flow validation UI: call validate API, highlight invalid nodes/edges in red
- [x] Create flow list page: list all flows, click to open in editor
- [x] Support multiple flows (one per inbound number group or reusable)
