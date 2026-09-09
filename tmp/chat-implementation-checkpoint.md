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
