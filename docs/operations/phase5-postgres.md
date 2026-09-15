# Phase 5 PostgreSQL integration

Checkpoint: 15 September 2026. Migrations: `00010_team_matches.sql`, `00011_match_lifecycle.sql`. ADR: `0020-internal-team-match.md`.

## What is available

The existing PostgreSQL repository now exposes trusted `CreateMatch`, `LoadMatch`, and `StartMatch` operations. Create consumes a validated waiting domain configuration and two already prepared waiting tables; all eight unique session identities must match the room seats and readiness. Match identity/configuration is idempotent. The authenticated lifecycle service creates fresh rooms and obtains all eight readiness approvals through the Team Match UI.

Start checks owner and match revision, generates the entire board set, persists immutable sources, and starts both room boards in one transaction. Both rooms retain their own table/board IDs and event sequences. `match_room_boards` binds them to shared match-board IDs. Dealer and vulnerability use the canonical board-number cycle. No caller-supplied gameplay score enters the ledger.

The existing command repository handles subsequent ready changes, legal play, claims/undo, next board, and final room closure. Next board selects the persisted source and never invokes the casual random generator. A room can finish all of its boards while its partner room remains on board one. Finish is accepted only after the last board; starting extra boards is rejected. Casual start, seating changes, replacement, bots, leaving, and expiry cannot bypass the match lifecycle.

Scored boards remain undoable until next-board/finish. That transaction creates and validates the permanent archive, adds the unique room result, computes any paired IMP comparison and total, updates status/revision, and persists the table outcome. Any failure rolls everything back. Identical request/archive delivery cannot add another result or increment totals. Conflicting final data is rejected; database triggers protect shared sources, sealed board scores, result rows, and comparisons from updates.

`LoadMatch` is private recovery state, not a response DTO. It uses repeatable-read consistency and validates normalized data through the domain, including recomputation of persisted IMP totals. Use the domain's participant projection at the authenticated service boundary. Replay/archive/DDS reads additionally require match completion and original room membership. Tables use RLS with no client access policies.

## Deployment order

1. Apply migrations 00010 and 00011 through the existing approved migration workflow.
2. Deploy an API build that includes the match command/privacy guards. Readiness now rejects databases without the match schema.
3. Deploy the matching web build after API readiness passes. Retain the independent bridge review, compatible hosting, and closed-beta drill release gates.

No Supabase migration or production deployment was performed for this checkpoint. Single-instance hosting and independent bridge review remain release gates.

## Recovery and rollback

Reopen the PostgreSQL repository and call `LoadMatch` for match state, and the existing `FindTable`/actor snapshot recovery for each room. A persisted comparison is never applied incrementally during recovery. Duplicate start returns current state with no generated deals or initial event batches. If the initial commit succeeded but publication was lost, the lifecycle service must refresh both room snapshots; do not call the deal source again.

A source-generation or SQL failure before start commit leaves both waiting tables and all shared source rows unchanged. Failure while closing a board leaves its previous scored snapshot, undo state, events, and match ledger unchanged; retry the same table request ID after recovery.

Do not run migration down on live match data. Down removes the match ledger/bindings and its safeguards. After match creation is enabled, an application rollback target must retain migration-00010/00011 compatibility and match lifecycle/privacy guards. An older casual-only API must not serve bound rooms. Pause service instead if no compatible rollback build exists. Waiting cancellation is supported. Active cancellation, owner succession/substitution, and inactivity expiry remain unavailable. Migration 00011 down rejects retained CANCELLED rows; do not use down as an application rollback.

## Local validation

An isolated PostgreSQL 17 container/database was used; the existing local chat database was not changed. Migration up/down/up and sqlc regeneration passed. All test-created matches/tables were cleaned up by the integration fixtures.

```sh
make migrate-validate
make generate-db
go test -race ./apps/api/...
go vet ./apps/api/...
make lint-api
TEST_DATABASE_URL="$LOCAL_TEST_DATABASE_URL" go test -race -tags=integration ./apps/api/internal/database -count=1
TEST_DATABASE_URL="$LOCAL_TEST_DATABASE_URL" go test -race -tags=integration ./apps/api/internal/realtime -count=1
```

Database integration passed in 35.144s. Final match scenarios passed in 11.272s; realtime integration passed in 1.205s. Match tests cover concurrent generation/finalization, readiness and revision fences, source/SQL rollback, independent progression, nonzero signed comparisons and zero results, undo before sealing, reopened-repository recovery, duplicate archive delivery, malformed totals, missing bindings, immutable final rows, fixed membership, cross-room privacy, and RLS/schema readiness. API race tests, Go vet, lint (0 issues), and migration validation passed.

These initial checks establish repository/database behavior. API and browser evidence is recorded below; production restart drills and hosting compatibility remain pending.


## API/actor and browser checkpoint — 15 September 2026

Migration 00011 supports waiting cancellation and owner/request deduplication. Create locks all invited accounts in stable order, creates two fresh rooms and enforces at most four pending/active invitations per account. The caller must occupy one seat; all eight readiness approvals are explicit. Any participant may cancel while waiting. Active abandonment and owner succession remain operational limitations.

API/actor race, vet and lint PASS. `TestMatchHTTPEightClientActorRecovery` PASS (47.383s): eight isolated WebSockets, raw recipient hand/session/room privacy, authenticated REST lifecycle, duplicate/conflicting creates, owner/revision fences, lost notification and retry refresh, independent two-board finalization, and waiting cancellation. PostgreSQL match race suite PASS (21.198s).

Playwright `e2e/team-match.spec.ts` PASS (final run 1.9m): eight registered browser contexts, roster selection, consent, start, own-room auction, passed-out boards, final sealing and persisted IMP after reload. Setup/table/result screenshots and overflow checks cover 320/390/768/1024/1440/1920 px; clicks use real Playwright pointer actions. This smoke validates a passed-out match; full bridge-rule validation remains in engine and existing gameplay tests. Browser fixtures remain only in the isolated local test database; integration fixtures clean up their own records.

Production web build, TypeScript, 67 web unit tests and contract tests PASS. Web lint: zero errors, two pre-existing aria-description warnings. No production migration/deployment, operational restart drill or pilot has been performed.

Migration 00011 up/down/up also PASS on a newly created disposable local database. The database was dropped after verification; the browser database and unrelated chat database were not altered by this drill.


## Local hardening — 16 September 2026

Lifecycle rollback/capacity/discovery and real-process restart/database-outage/compatible-rollback checks now PASS. Four two-board matches and 32 player sockets recovered and completed with identical final results under build `0200467`. Security scans and focused race/vet/lint checks pass. The list prioritizes unfinished matches, and authentication treats infrastructure errors as retryable 503. See `docs/operations/phase5-hardening.md` for exact scope, timings, reproduction, fixture corrections and remaining release gates. No production migration or deployment was performed.
