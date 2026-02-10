# Sprint 04 — Admin UI Shell

**Phase**: 1A (Foundation)
**Focus**: React project setup, login, setup wizard, layout, initial CRUD pages
**Dependencies**: Sprint 03

**PRD Reference**: Section 6.1 (Pages/Views), Section 6.3 (Embedded SPA)

## Tasks

- [x] Initialize Vite + React 18 + TypeScript project in `web/`
- [x] Install and configure Tailwind CSS
- [ ] Set up React Router with route definitions for all pages
- [ ] Create API client module (`web/src/api/`) with auth handling, JSON envelope parsing
- [ ] Create layout components: sidebar nav, header, content area
- [ ] Create Login page — username/password form, session auth
- [ ] Create Setup Wizard UI — multi-step form (admin account, hostname, SIP ports, first trunk, first extension, license key)
- [ ] Create Dashboard page with placeholder stats (active calls, registrations, recent CDRs)
- [ ] Create Extensions CRUD page — list, create, edit, delete
- [ ] Create Trunks CRUD page — list, create, edit, delete, status indicator
- [ ] Create Voicemail Boxes CRUD page — list, create, edit, delete
- [ ] Create shared form components (text input, select, toggle, number input)
- [ ] Create shared table component with pagination
- [ ] Wire up embed: add `web/dist/` placeholder so `//go:embed` compiles
