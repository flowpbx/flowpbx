# Sprint 11 — Flow Engine Core

**Phase**: 1C (Flow Engine & Nodes)
**Focus**: Graph walker, CallContext, node handler interface, flow validation
**Dependencies**: Sprint 08, Sprint 09

**PRD Reference**: Section 5.3 (Flow Engine), Section 5.2 (Flow Graph JSON Structure), Section 4.10 (Call Flows)

## Tasks

- [x] Create `internal/flow/context.go` — CallContext struct (SIP transaction, caller info, collected DTMF, variables, traversal path)
- [x] Create `internal/flow/engine.go` — graph walker: load published flow JSON → resolve entry node → walk graph
- [x] Define node handler interface: `Execute(ctx *CallContext, node Node) (outputEdge string, err error)`
- [x] Implement node-to-entity resolution: load entity by `entity_id` + `entity_type` from node data
- [x] Implement edge following: after node execution, find output edge by handle name → resolve next node
- [x] Implement per-node timeout handling
- [x] Implement error handling: if node execution fails, attempt graceful call termination
- [x] Record flow traversal path in CDR (`flow_path` field — JSON array of node IDs visited)
- [x] Create `internal/flow/validator.go` — validate flow graph: check for disconnected nodes, missing entity references, orphan paths
- [x] Wire flow engine into inbound call handling: DID match → find flow_id + flow_entry_node → spawn flow walker goroutine
- [x] Create call flow CRUD API: `GET/POST/PUT/DELETE /api/v1/flows`
- [x] Implement `POST /api/v1/flows/:id/publish` — snapshot current flow_data, set published=true
- [x] Implement `POST /api/v1/flows/:id/validate` — run validator, return errors/warnings
