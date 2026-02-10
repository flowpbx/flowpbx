# Sprint 07 — RTP Media Proxy

**Phase**: 1B (Call Handling)
**Focus**: RTP relay, SDP manipulation, codec passthrough, DTMF, NAT handling
**Dependencies**: Sprint 05

**PRD Reference**: Section 8.5 (Media Proxy / RTP Engine)

## Tasks

- [x] Create `internal/media/proxy.go` — UDP socket pool for RTP relay (configurable port range, default 10000-20000)
- [x] Implement RTP session allocation: allocate a pair of ports (RTP + RTCP) per call leg
- [x] Implement SDP parsing (extract media lines, codecs, connection info)
- [x] Implement SDP rewriting (replace endpoint IPs/ports with proxy addresses)
- [x] Implement G.711 alaw (PCMA, payload 8) passthrough relay
- [x] Implement G.711 ulaw (PCMU, payload 0) passthrough relay
- [x] Implement Opus (payload 111) passthrough relay
- [x] Create `internal/media/dtmf.go` — RFC 2833 telephone-event relay
- [x] Implement SIP INFO DTMF fallback detection
- [x] Implement symmetric RTP / NAT handling (learn remote port from first packet)
- [x] Implement session timeout and cleanup for orphaned RTP streams
- [x] Create media session lifecycle: create → start relay → stop → release ports
- [ ] Add RTP packet counters and basic stats per session (for future metrics)
