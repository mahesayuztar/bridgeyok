# Chat and realtime notification validation

Completed: 11 September 2026. Branch: main. Implementation history: b330024, 01a01fe, 159fa08, 1adde36, followed by focused isolation and validation commits. This report replaces the unfinished checkpoint.

| Job | Status | Evidence |
| --- | --- | --- |
| JOB-01 | PASS | PostgreSQL integration: mutual Friends authorization, offline persistence, cross-user/table isolation, revoked access, request dedupe/conflict, cursor pagination, 90/14-day expiry and cleanup. Local daily pg_cron job installed; temporary probe executed the same cleanup successfully three times and was removed. |
| JOB-02 | PASS | Existing Go socket sends recipient-scoped messages and authoritative ACKs; protocol tests reject sender/revision spoofing. Contract generation and four contract tests pass. Chat does not change game revision. |
| JOB-03 | PASS | Five client tests cover Unicode graphemes, optimistic reconciliation, failed/retrying states with identical request identity, stale ACK/replay, notification dedupe and account/socket isolation. Browser delays ACK 500 ms and asserts optimistic rendering at 150 ms. |
| JOB-04 | PASS | Real Friends A/B realtime chat; receiver page closed before offline send and reopened to retained history. Eight responsive widths, emoji, Open Chat, 50+50 history pagination, preserved reading position and new-message indicator verified. |
| JOB-05 | PASS | Collapsible desktop sidebar and mobile sheet tested at 320, 375, 390, 768, 1024, 1366, 1440 and 1920 px. Auction/bid, dummy/current trick/own hand, close/open geometry and overflow checked. Real mouse drag plays a card with desktop chat open; CDP touch drag plays a card after closing the mobile sheet. |
| JOB-06 | PASS | Follow View User, mutual Chat, invite Join, private/table Open Chat, including cross-page navigation, pass browser checks. Self/replayed/open-panel events suppress toasts. Integration verifies invite storage removal; invitations endpoint returns an empty list. Notifications have no durable inbox or offline queue. |
| JOB-07 | PASS | Bounded P0/P1/P2/P3 scheduler and blocking-store flood tests pass: 50 chat sends do not block a table command or change game revision. Existing slow-consumer recovery retained. Server online invite checks and offline private delivery tested. |
| JOB-08 | PASS | Focused API race/integration, vet/lint, migration validation, contracts, client tests, production web build, TypeScript and browser matrix pass. |

## Verification runs

- API race suites: realtime, httpapi, config and chat; Go vet and API lint pass (zero issues).
- PostgreSQL integration: `go test -race -tags=integration ./apps/api/internal/database -run 'TestChat|TestRegisteredAccountSocialAndInviteBoundary' -count=1` passes against the isolated local PostgreSQL 17 database with migrations through 00009 (42.966 seconds).
- `make migrate-validate` and four contract tests pass.
- Web production build and TypeScript pass. ESLint has no errors and two pre-existing aria-description warnings in score sheet/table status controls.
- `node --test apps/web/app/chat-state.test.mjs apps/web/app/chat-store.test.mjs`: five tests pass.
- All four focused browser scenarios pass (1.2 minutes). After socket isolation changes, private/table regressions pass (21.9/15.9 seconds), and social cross-page navigation passes (29.2 seconds). Final table run including actual CDP touch drag passes (18.9 seconds).
- Browser keyboard coverage uses a native textarea, visualViewport handling, and reduced viewport height emulation. Physical mobile OS keyboard behavior was not tested on a hardware device.

## Deployment prerequisites and limits

Production was not deployed or modified. Apply migrations `00008_chat.sql` and `00009_ephemeral_invites.sql`, then install `db/operations/chat-retention.sql` in the target database with pg_cron enabled. Migration 00009 removes legacy persisted invitations; its down migration restores only an empty compatibility table. The named daily cleanup was installed and executed locally, not in production.

Keep the existing Go WebSocket deployment and origin configuration. No Redis, Supabase Realtime, or second socket service was introduced. Retained chat is persisted; actionable notification events are live-only.

Architecture: `docs/adr/0019-chat-and-ephemeral-realtime.md`. Main boundaries: API chat domain/repository/realtime handlers, shared web chat store/panel, existing account presence and gameplay socket. Regression coverage: `apps/web/e2e/chat.spec.ts`, chat state/store tests, API chat/realtime/database tests, and contracts tests.

Remaining implementation work: none for JOB-01–08. Deployment and physical-device testing are not claimed by this local validation report.
