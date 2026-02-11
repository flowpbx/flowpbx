# Learnings

Runtime discoveries and gotchas captured during build iterations.

## Project Setup
<!-- Package manager, build tools, etc. -->

## Code Patterns

### API pagination envelope
List endpoints consumed by the frontend `list()` helper (in `web/src/api/client.ts`) **must** return a `PaginatedResponse` struct, not a raw array. The response shape after envelope unwrapping must be:
```json
{"items": [...], "total": N, "limit": N, "offset": N}
```
Use `parsePagination(r)` from `response.go` to extract `limit`/`offset` query params, then wrap results with `PaginatedResponse{Items, Total, Limit, Offset}`.

If the store `List()` method returns all records (no DB-level pagination), slice in-memory:
```go
total := len(all)
start := pg.Offset
if start > total { start = total }
end := start + pg.Limit
if end > total { end = total }
writeJSON(w, http.StatusOK, PaginatedResponse{Items: all[start:end], Total: total, Limit: pg.Limit, Offset: pg.Offset})
```

Endpoints that are **not** consumed via the frontend `list()` helper (e.g. time switches, ring groups, IVR menus, prompts, flows, conferences) return raw arrays and that's correct — their frontend pages use `get()` directly.

## Mobile App

### SIP library selection: siprix_voip_sdk

**Decision**: Use `siprix_voip_sdk` (pub.dev) as the Flutter SIP library.

**Rationale**: After evaluating all available Flutter SIP options (dart_sip_ua, siprix_voip_sdk, flutter_pjsip, sip_native, ABTO VoIP SDK), siprix_voip_sdk is the only viable choice:

- **Architecture match**: Native SIP-over-TLS + SDES-SRTP matches FlowPBX's traditional SIP/RTP stack. No WebSocket/WebRTC adapter layer needed on the Go side.
- **Feature completeness**: TLS, SRTP, Opus, G.711, DTMF, push wake-up (PushKit + FCM), CallKit/ConnectionService, background calls — all built in.
- **dart_sip_ua rejected**: WebRTC-centric (ported from JsSIP). Would require adding WSS + DTLS-SRTP support to the Go SIP stack — major architecture change for no benefit.
- **flutter_pjsip rejected**: Abandoned (v0.0.2-dev, last updated Jan 2024), incomplete FFI bindings.
- **sip_native rejected**: Android-only, uses deprecated `android.net.sip` API.
- **ABTO VoIP SDK rejected**: Also commercial but less community visibility than Siprix; no advantage.

**Trade-off**: Siprix requires a commercial license (trial limits calls to 60 seconds). The native SDK is closed-source. This is acceptable because:
1. FlowPBX itself is a commercial product (license server in the architecture).
2. Siprix license is one-time, not subscription.
3. No viable open-source Flutter SIP library exists that supports TLS + SDES-SRTP + push wake-up.

**Fallback**: If the commercial dependency becomes untenable, the PRD documents a fallback path: custom PJSIP integration via platform channels (native Swift/Kotlin SIP engines with Flutter UI). This would require significant effort but removes the proprietary dependency.

## Common Pitfalls

### Raw array vs PaginatedResponse mismatch
If a frontend page uses `listFoo()` which calls the `list()` client helper, the Go handler **must** return `PaginatedResponse`. Returning a raw array causes the frontend to get `undefined` for `res.items`, which crashes `DataTable` with `Cannot read properties of undefined (reading 'length')`. Always check the frontend API module to see whether it uses `list()` (paginated) or `get()` (raw) before writing a new list handler.
