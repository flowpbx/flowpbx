# Sprint 07 — RTP Media Proxy

**Phase**: 1B (Call Handling)
**Focus**: RTP relay, SDP manipulation, codec passthrough, DTMF, NAT handling
**Dependencies**: Sprint 05

**PRD Reference**: Section 8.5 (Media Proxy / RTP Engine)

## Tasks

- [x] Create `internal/media/proxy.go` — UDP socket pool for RTP relay (configurable port range, default 10000-20000)
- [x] Implement RTP session allocation: allocate a pair of ports (RTP + RTCP) per call leg
- [ ] Implement SDP parsing (extract media lines, codecs, connection info)
- [ ] Implement SDP rewriting (replace endpoint IPs/ports with proxy addresses)
- [ ] Implement G.711 alaw (PCMA, payload 8) passthrough relay
- [ ] Implement G.711 ulaw (PCMU, payload 0) passthrough relay
- [ ] Implement Opus (payload 111) passthrough relay
- [ ] Create `internal/media/dtmf.go` — RFC 2833 telephone-event relay
- [ ] Implement SIP INFO DTMF fallback detection
- [ ] Implement symmetric RTP / NAT handling (learn remote port from first packet)
- [ ] Implement session timeout and cleanup for orphaned RTP streams
- [ ] Create media session lifecycle: create → start relay → stop → release ports
- [ ] Add RTP packet counters and basic stats per session (for future metrics)
