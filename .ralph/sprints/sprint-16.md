# Sprint 16 — Remaining Admin UI CRUD Pages

**Phase**: 1C (Flow Engine & Nodes)
**Focus**: Ring groups, IVR menus, time switches, conferences, recordings, settings pages
**Dependencies**: Sprint 04, Sprint 15

**PRD Reference**: Section 6.1 (Pages/Views), Section 7 (REST API)

## Tasks

- [x] Create Ring Groups CRUD page + API: `GET/POST/PUT/DELETE /api/v1/ring-groups`
- [x] Create IVR Menus CRUD page + API: `GET/POST/PUT/DELETE /api/v1/ivr-menus`
- [x] Create IVR digit mapping editor UI (0-9, *, #, timeout, invalid)
- [ ] Create IVR audio prompt upload / select from library in UI
- [ ] Create Time Switches CRUD page + API: `GET/POST/PUT/DELETE /api/v1/time-switches`
- [ ] Create time switch visual rule editor: day checkboxes + time range pickers
- [ ] Create time switch weekly grid preview
- [ ] Create time switch holiday/specific date override support
- [ ] Create timezone selector component
- [ ] Create Conference Bridges CRUD page + API: `GET/POST/PUT/DELETE /api/v1/conferences`
- [ ] Create Recordings browser page: list, search, play, download, delete
- [ ] Create Recordings API: `GET /api/v1/recordings`, `GET .../download`, `DELETE`
- [ ] Create Settings page: SIP ports, TLS certs, codecs, recording storage, SMTP, license key, push gateway URL
- [ ] Implement `GET/PUT /api/v1/settings` — system config API
- [ ] Implement `GET /api/v1/system/status` — SIP stack status, trunk registrations
- [ ] Implement `POST /api/v1/system/reload` — hot-reload config without restart
