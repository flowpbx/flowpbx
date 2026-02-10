# Sprint 10 — CDR System

**Phase**: 1B (Call Handling)
**Focus**: Call detail records, hangup cause mapping, search/filter, CSV export
**Dependencies**: Sprint 08

**PRD Reference**: Section 4.11 (CDR), Section 7 (CDR API), Section 6.1 (CDR page)

## Tasks

- [x] Create CDR creation on call start (insert row with call_id, start_time, caller/callee, direction)
- [x] Update CDR on answer (set answer_time)
- [x] Update CDR on hangup (set end_time, duration, billable_dur, disposition, hangup_cause)
- [x] Implement hangup cause mapping: SIP response codes → friendly labels (answered, no_answer, busy, failed, voicemail)
- [x] Track recording_file path in CDR when call is recorded
- [x] Reserve flow_path field (JSON array of node IDs, populated later by flow engine)
- [x] Create CDR API: `GET /api/v1/cdrs` with pagination, search, date range filter, direction filter
- [x] Create `GET /api/v1/cdrs/:id` — single CDR detail
- [x] Implement `GET /api/v1/cdrs/export` — CSV export with date range filter
- [x] Create CDR / Call History page in admin UI — searchable, filterable table with date range picker
- [x] Add recent CDRs widget to Dashboard page
