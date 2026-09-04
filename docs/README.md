# BridgeYok Documentation Index

## Authority order

1. [`AGENTS.md`](../AGENTS.md) — permanent repository rules.
2. [`PLAN.md`](../PLAN.md) — accepted roadmap dan decision register.
3. [`product/product-contract.md`](product/product-contract.md) — normative focused bridge product/rules behavior.
4. ADR — accepted architecture decisions.
5. OpenAPI, JSON Schema, migrations, dan tests yang dibuat Phase 1+.

Jika dua dokumen setingkat bertentangan, dokumen dengan change record/ADR paling baru berlaku setelah semua references diperbarui.

## Phase 0 artifacts

| Artifact | Purpose | Status |
|---|---|---|
| [`product/product-contract.md`](product/product-contract.md) | role, lifecycle, recovery, ruleset, scoring, hidden-information, error, acceptance contract | Accepted |
| [`product/wbf-compliance-matrix.md`](product/wbf-compliance-matrix.md) | klasifikasi WBF Law 1–93, Director boundary, dan executable evidence | Engineering complete; bridge review pending |
| [`product/wireflows.md`](product/wireflows.md) | end-to-end happy/degraded/recovery UX flows | Accepted |
| [`security/threat-model.md`](security/threat-model.md) | trust boundaries, data classification, STRIDE-style threats, controls/tests | Accepted |
| [`observability/telemetry-contract.md`](observability/telemetry-contract.md) | on-call questions, log/metric/trace/product-event contract, alerts | Accepted |
| [`adr/0001-modular-monolith.md`](adr/0001-modular-monolith.md) | deployable/module boundary | Accepted |
| [`adr/0002-postgresql-authoritative-state.md`](adr/0002-postgresql-authoritative-state.md) | durable state/write/recovery semantics | Accepted |
| [`adr/0003-native-websocket-protocol-v1.md`](adr/0003-native-websocket-protocol-v1.md) | realtime transport/protocol/reconnect/backpressure | Accepted |
| [`adr/0004-guest-identity-and-ws-ticket.md`](adr/0004-guest-identity-and-ws-ticket.md) | guest credential, recovery, handshake auth | Accepted |
| [`adr/0005-defer-redis-until-scale-out.md`](adr/0005-defer-redis-until-scale-out.md) | current-roadmap no-Redis boundary | Accepted |
| [`adr/0007-consensus-claim-and-undo.md`](adr/0007-consensus-claim-and-undo.md) | durable consensus claim/undo baseline | Accepted |
| [`adr/0008-simple-table-bots.md`](adr/0008-simple-table-bots.md) | owner-managed deterministic seat bots | Accepted |
| [`adr/0009-optimistic-gameplay-client-and-presentation.md`](adr/0009-optimistic-gameplay-client-and-presentation.md) | optimistic client, sequenced presentation, canonical card, UX-G1 engine gate | Accepted |
| [`adr/0010-automatic-controller-handoff.md`](adr/0010-automatic-controller-handoff.md) | automatic newest-device controller handoff with epoch fencing | Accepted |
| [`adr/0013-recipient-scoped-play-history.md`](adr/0013-recipient-scoped-play-history.md) | least-privilege completed-trick projection and public progress count | Accepted |

## Decision traceability

| Decision | Normative elaboration | Architecture/test evidence target |
|---|---|---|
| OD-01 multi-board/finish | Product Contract 7.6 | table lifecycle integration/E2E |
| OD-02 seat recovery | Product Contract 5.2, 7.5; Wireflows 14 | ADR-004; recovery/fencing E2E |
| OD-03 TTL/retention | Product Contract 7.7 | cleanup/expiry integration |
| OD-04 owner succession | Product Contract 7.4; Wireflows 6 | concurrent claim test |
| OD-05 completed deal visibility | Product Contract 12 | projection privacy matrix |
| OD-06 dealer/vulnerability cycle | Product Contract 8.1 | 16-board golden test |
| OD-07 canonical score | Product Contract 11 | score golden/property tests |
| OD-08 React Aria + Tailwind | Product Contract UX acceptance; Wireflows | Phase 1 dependency lock, accessibility tests |
| OD-09 credential storage/ticket | Product Contract 5; Threat Model T-001–T-008 | ADR-004; auth/replay/revoke tests |
| OD-10 Go router/WS library | ADR-001, ADR-003 | Phase 1 dependency/bootstrap test |
| OD-11 no third-party analytics | Telemetry Contract | telemetry privacy inspection |
| OD-12 always free/non-commercial | Product Contract 2; Wireflows capacity states | closed-beta admission tests |
| OD-13 closed-beta scope | PLAN.md phase summary | phase regression review |
| OD-14 WBF compliance boundary | Product Contract section 14; WBF Compliance Matrix | `TestWBFComplianceMatrix`; experienced-player review pending |
| OD-15 DDS boundary | Product Contract DDS section | analysis boundary tests |
| OD-16 team match | Product Contract Team Match section | paired-board integration |
| OD-17 simple table bots | Product Contract sections 3, 6, 7; ADR-008 | table/actor/protocol/projection tests |
| OD-18 gameplay UX reliability | `apps/web/PLAN.md` sections 19–23; ADR-003 amendment; ADR-009 | optimistic/realtime/component/pointer/visual browser matrix and UX-G1 |
| OD-19 automatic controller handoff | Product Contract sections 7.5 and 15; Wireflows 14; ADR-010 | reducer and two-tab/reload E2E plus realtime fencing tests |
| OD-20 recipient-scoped play history | ADR-013; `apps/web/PLAN.md` sections 22–23 | projector recipient matrix, snapshot round-trip, raw-frame and browser/mobile history tests |

## Phase 0 exit report

Date reviewed: 29 Agustus 2026.

| Exit gate | Evidence | Result |
|---|---|---|
| All closed decisions reflected in specs/contracts/tests | Decision traceability table above; Product Contract section 14 | PASS |
| State machine, permissions, scoring, retention, recovery, error behavior reviewed | Product Contract sections 6–14; WBF normative references | PASS |
| Threat model has owner/mitigation/test for high risks | Threat Model sections 7 and 11 | PASS |
| Realtime/auth/state choices are closed for Phase 1 | ADR-001 through ADR-005 | PASS |
| Critical wireflows and degraded states are explicit | Wireflows sections 3–18 | PASS |
| Product events contain no private payload | Telemetry Contract sections 3–7 | PASS |
| No Phase 1 work requires a new major architecture choice | Phase 1 inputs below | PASS |

Phase 0 status: **COMPLETE**.

Current roadmap after Phase 2 is intentionally narrow: Phase 3 stabilizes realtime table play, Phase 4 covers WBF compliance/deal provenance/DDS analysis, and Phase 5 delivers internal Team Match plus closed-beta hardening.

## Current agent context — Gameplay UX reliability

Objective GUX dan ENG-01 telah selesai. Use `apps/web/PLAN.md` sections 19–23 as the detailed objective tracker and root `PLAN.md` as the cross-component gate. Future implementation agents must preserve these reasons and boundaries:

- UX correctness, immediate feedback, predictable bridge-table convention, and spatial clarity outrank decorative novelty.
- Extract meaningful gameplay domains from the current large table component without atomizing wrappers. Own, dummy, and played cards use one canonical card primitive with context variants.
- A legal local call/play is optimistically projected, but Go/PostgreSQL remains authoritative. Authoritative, optimistic, and animation/presentation state are separate and reconcile by request/revision/sequence with snapshot recovery.
- Click/tap/drag share one command path. Known-illegal triggers are disabled; rule-invalidity toast is not the normal enforcement path.
- Card movement/trick pacing is functional feedback in one sequenced queue. Only a board click skips the current normal gameplay animation.
- Dummy, trick, participant, own-hand, and navbar zones must not overlap from 320 px mobile through desktop. Persistent gameplay actions use navbar space before floating panels; active copy stays concise.
- ENG-01 history is complete under ADR-013/OD-20. ENG-02 score sheet remains blocked on semantics, and ENG-03 bot consensus remains a separate deliberate supersession of ADR-008.

Repository risks to re-check before implementation: single-table IMP semantics are unresolved; ADR-008 currently disables consensus with any bot. ACK/rebase handling and recipient-scoped `CompletedTricks` are implemented and covered by delayed realtime/raw-frame tests.

Rules were source-reviewed against the current WBF 2017 Laws page, including the revisions effective 1 Januari 2024 and Law 77 scoring table. An independent experienced-player review remains an additional Phase 2 golden-test sign-off; it is evidence validation, not an open product decision.

## Phase 1 inputs now fixed

- Repository shape and module boundaries: ADR-001.
- Go HTTP router: `net/http` + `go-chi/chi/v5`.
- WebSocket library: `github.com/coder/websocket`.
- REST contract format: OpenAPI.
- Realtime contract format: JSON Schema, native WSS protocol v1.
- Database approach: PostgreSQL + pgx/sqlc + migrations, transaction per accepted mutation.
- MVP Redis dependency: none.
- UI foundation: Next.js Active LTS, TypeScript strict, React Aria Components, Tailwind CSS.
- Identity baseline: memory access token, IndexedDB device credential, 45-second one-time WS ticket.
- Telemetry baseline: structured logs/correlation first; bounded RED/USE metrics; no third-party product analytics.

## Change workflow

1. Describe the observed requirement/problem.
2. Identify affected decision, product contract, threat, telemetry, protocol, and migration.
3. Add/supersede ADR when architecture changes.
4. Update normative docs and contracts in the same change.
5. Add/adjust acceptance and regression tests.
6. Record date and compatibility/rollout consequences.
