# Chat implementation checkpoint

Last updated: 2026-09-10 (continued after user “lanjutkan”)
Branch: main. Latest committed implementation: 01a01fe; baseline 8c14555; checkpoint b330024.

| Job | Status | Remaining evidence/work |
| --- | --- | --- |
| JOB-01 | IN_PROGRESS | Cron schedule installation/execution; richer concurrency/retention boundary tests |
| JOB-02 | IN_PROGRESS | Final contract generation/tests; socket delivery/privacy/reconnect tests |
| JOB-03 | IN_PROGRESS | ACK/reconnect/Unicode unit tests PASS; stronger delayed-ACK browser assertion and retry tests |
| JOB-04 | IN_PROGRESS | Private browser flow PASS; older-history scroll and new-message indicator coverage; accessibility polish |
| JOB-05 | IN_PROGRESS | Table browser test running; responsive side panel/sheet geometry and actual pointer proof unfinished |
| JOB-06 | IN_PROGRESS | Follow/friend/invite socket events implemented; migration 00009 removes persisted invites; action/dedupe browser proof pending |
| JOB-07 | IN_PROGRESS | Bounded P0/P1/P2/P3 outbound channels, chat worker and global chat DB slot; flood blocking-store test PASS; scheduler test pending |
| JOB-08 | IN_PROGRESS | Finish focused test matrix and final build/lint/vet; no complete gate claimed |

Completed changes:
- Migration 00008 chat messages, unique(sender,request), indexes, RLS, cleanup function; private 90 days/table 14 days enforced in reads too.
- Repository authorizes Friends both directions and active table membership in transaction, detects reused request content/target conflicts, keyset pages, recipient-scoped socket publication.
- Existing Go socket handles chat.private.send/chat.table.send with no game revision; chat.accepted ACK control and chat.*.received control.
- Web store projects immediate messages with sending/sent/failed/retrying, identity reconciliation and retry; shared with gameplay socket. Account pages connect to the same Go endpoint. No new realtime infrastructure.
- Friends Chat entry, shared ChatPanel, basic table toggle/panel; native emoji textarea and grapheme validation.
- Realtime-only chat/social toast and Open Chat/View User/Chat/Join/Dismiss actions. Legacy invitations endpoint now returns empty; no writes to invite storage.

Migration/schema state:
- Isolated PostgreSQL17 Docker container bridgeyok-chat-db, localhost:55432, DB bridgeyok/user postgres/local test password chat-local; migrations through 00009 applied. Production untouched.
- db/operations/chat-retention.sql schedules 03:17 daily with pg_cron; extension package installed inside local container but preload/database settings and cron execution not yet verified. Container restart must wait for active browser tests.
- Original apt install session 2024; HTTPS retry 99142 finished with apt certificate warnings but pg_cron extension files exist. Check processes before another apt operation.

Tests already passed:
- go test -race ./apps/api/internal/chat ./apps/api/internal/realtime ./apps/api/internal/httpapi ./apps/api/internal/config (latest suite before global chatSlots addition; subsequent session 72944 underway).
- Go vet API passed before latest scheduler/global-slot edits; lint pending latest.
- TEST_DATABASE_URL=postgres://postgres:chat-local@127.0.0.1:55432/bridgeyok?sslmode=disable go test -race -tags=integration ./apps/api/internal/database -run 'TestChat|TestRegisteredAccountSocialAndInviteBoundary' -count=1 PASS (20.639s) with migrations 00008/00009.
- node --test apps/web/app/chat-state.test.mjs: 2 PASS (reconciliation/stale timeout, Unicode).
- Web ESLint: no errors; only two pre-existing aria-description warnings.
- Private browser test PASS (13.4s; total28.5s), log tmp/chat-browser.log: A↔B, offline retained message, emoji, toast Open Chat, widths320/375/390/768/1024/1366/1440/1920.
- Blocking chat DB + 50 sends does not block table command; no game revision change from chat; spoof/revision protocol tests PASS.

Known issues / active work:
- Table browser test process session52258, log tmp/chat-table-browser.log, started ~23:48 local; no pass yet. It may be stuck on pointer interception by panel before screenshot loop. Inspect failure trace once finished. Do not restart DB until it finishes.
- Its final drag selector is currently wrong/conditional and must become mandatory using existing button[aria-label^="Mainkan "]:enabled and .board-play-zone, with card-removal assertion.
- First contract test hit missing object types under if/then; fixed. Regeneration/test session93322 pending.
- Earlier typecheck had one signal optional-type error (fixed) plus conflicting stale .next and .next-e2e generated route validators. Refresh generated artifacts rather than changing tsconfig.
- apps/web/next-env.d.ts changed automatically by e2e; restore original generated target after build.
- UI code still requires review, readable formatting, desktop panel layout improvement, keyboard/sheet behavior, sender/presence and notification action evidence.
- Durable checkpoint is force-tracked because tmp and Markdown are ignored. Only first persistence commit exists; partition pending backend/protocol/frontend/test changes into focused commits.

Important files:
apps/api/internal/chat/*; database/chat_repository*; database/social_events.go; database/account_repository.go; realtime/chat*; realtime/server.go/protocol.go; httpapi/account_handlers.go/router.go; config/config.go; db/migrations/00008_chat.sql/00009_ephemeral_invites.sql; db/operations/chat-retention.sql; packages/contracts/websocket/envelope.schema.json; apps/web/app/chat-*; account-presence.tsx; social-users.tsx; table/table-chat.tsx; table/table-status-bar.tsx; use-table-session.ts; e2e/chat.spec.ts.

Next cheapest commands:
- tail -60 tmp/chat-table-browser.log
- inspect wait results for sessions72944 and93322 if available; otherwise process/log check.
- Complete cron local installation and test, then mark JOB-01 with actual evidence.
- Finish failing table geometry, then notification and scroll tests. Preserve original user scope in tmp/chat-request.txt.

Latest evidence (23:56 local):
- JOB-01 PASS: named 03:17 cron job installed; a temporary one-second probe invoked the same cleanup function successfully three times, then was unscheduled. Daily job remains installed locally. PostgreSQL config preloads pg_cron, database bridgeyok, background workers on.
- Latest API race suites (realtime/httpapi/config/chat), Go vet, and full API lint PASS (0 issues).
- Contract generation and 3 contract tests PASS after strict object-type correction.
- Table browser PASS (12.3s, total20.5s): active auction send + bid, dummy revealed, input/geometry at eight requested sizes, close/open restores own-hand vertical geometry, real pointer drag removes one card while desktop chat stays open. Screenshots inspected at320 and1440; desktop uses a reserved sidebar, mobile uses a nonfullscreen sheet. Original failure was Chat inside the closed table menu; moved to persistent navbar.
- Additional social/table recipient test is running as session35913, log tmp/chat-social-browser.log.
- Backend/protocol commit includes ADR0019 and PLAN update; production unchanged. Remaining frontend: keyboard/scroll/retry/dedupe tests, presence polishing, final build and UI commit.
