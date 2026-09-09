# Chat implementation checkpoint

Last updated: 2026-09-10
Branch: main; baseline: 8c14555

JOB-01 IN_PROGRESS
JOB-02 TODO
JOB-03 TODO
JOB-04 TODO
JOB-05 TODO
JOB-06 TODO
JOB-07 TODO
JOB-08 TODO

Completed: scoped architecture inspection. Existing uniseg dependency supports server grapheme validation. Existing account identity maps user to stable guest session. Existing invite notices poll persisted player_invites; replace notification delivery with ephemeral socket events.
Remaining: all implementation and validation jobs.
Important files: database/account_repository.go, realtime/server.go, realtime/protocol.go, httpapi/account_handlers.go, web/app/account-presence.tsx, social-users.tsx, use-table-session.ts.
Schema: migrations through 00007 baseline; no chat schema applied.
Tests passed: none for chat yet.
Known failures: none assessed.
Next cheapest check: focused chat domain unit tests, isolated PostgreSQL migration/integration tests.
Continuity: this checkpoint is explicitly force-tracked because tmp and Markdown are ignored. Full request retained in tmp/chat-request.txt locally.

2026-09-10 persistence checkpoint:
- JOB-01 IN_PROGRESS: migration 00008, chat domain/repository, history retention, idempotency and authorization implemented. Daily cron SQL provided at db/operations/chat-retention.sql; pg_cron execution still unverified.
- JOB-02 IN_PROGRESS: semantic socket commands and control ACK/receive implemented; history route wired. No browser integration yet.
- JOB-07 IN_PROGRESS: bounded per-connection chat worker and low-priority outbound queue; dedicated flood/priority tests still required. P1/P3 classification incomplete.
- Isolated PostgreSQL 17 container bridgeyok-chat-db at localhost:55432, database bridgeyok, user postgres, local-only password chat-local. Migrations through 00008 applied successfully. Production untouched.
- PASS: go test -race ./apps/api/internal/chat ./apps/api/internal/database; existing realtime/httpapi race suites; TEST_DATABASE_URL=postgres://postgres:chat-local@127.0.0.1:55432/bridgeyok?sslmode=disable go test -race -tags=integration ./apps/api/internal/database -run '^TestChat' -count=1.
- Integration proves offline message/history, two-way Friends authorization, private/table isolation, retry identity/conflict, pagination, expiry filtering and actual 90/14-day deletion.
- Go vet passes; lint identified De Morgan simplification, fixed; rerun focused lint.
