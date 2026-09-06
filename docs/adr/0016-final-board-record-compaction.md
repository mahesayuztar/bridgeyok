# ADR 0016 — Final board records and event compaction

Status: accepted, 2026-09-06

## Decision

Keep private `game_snapshots` upserted for every accepted command. Recovery continues exclusively from that snapshot. Keep seq/revision allocation, request deduplication, and publish-after-commit unchanged.

A scored board is still undoable. The finality boundary is an accepted transition to another board or to `FINISHED`, provided the previous engine state is `BOARD_SCORED`. An expired/abandoned unfinished board is not compacted. A scored board left open retains events until a next-board, finish, last-owner-leave, or expiry transition closes it.

At this boundary, the existing command transaction builds one versioned JSON `board_records` row. Searchable metadata/result/lineup stay in `boards` and `board_seat_attributions`, joined by board ID. The JSON preserves original deal/provenance and chronological revision batches (first seq, timestamp, typed events), including rejected claims, consensus votes, and undone actions. Per-event table ID, revision, timestamp, and sequence duplication is removed. PostgreSQL JSONB storage handles compression; no service, queue, worker, or scheduler is added.

Validation replays complete engine event batches with the existing pure `bridge.Reduce`; accepted undo restores the state before the latest action. The SHA-256 of the complete replayed engine state must match the canonical final snapshot, covering auction, play, claim, result, and ruleset. Archive data is read back and revalidated before deleting precisely its `BOARD_STARTED` through pre-transition last-seq range. A mismatch or persistence failure rolls back the entire command. Identical retries reuse the archive; conflicting data is never overwritten. The command processor logs transaction failures through its existing error path.

Retention for validated final-board events is immediate within the same transaction. The newly committed transition events remain available for WS publishing and reconnect. Active-board events and table lifecycle events outside the archived range remain untouched. Reconnect across a compacted range uses the existing contiguous-sequence check and recipient-projected snapshot fallback; it never substitutes private archive data into WS recovery. `CompletedBoardRecord` is a participant-authorized repository read for historical replay, not a new public endpoint.

## Rollout and limits

Apply migration 00006 before deploying the API. No existing event is deleted by the migration. Existing current scored boards compact on their next finalizing command. Older boards that were already superseded before deployment retain their events: there is no automatic historical backfill without their original final snapshot evidence. Waiting-table events and abandoned unfinished-board events retain existing behavior. This bounds completed-board event accumulation going forward, not every source of database growth (`processed_commands` and score sheets are outside this change).

The archive remains for the life of its parent board/table, following existing cascade semantics. Migration down removes archives and cannot restore previously deleted granular events; retain the migration on application rollback and back up the database before any schema rollback. Snapshot format is unchanged, so application rollback can still recover active games. Historical replay uses the deployed ruleset; changes to that ruleset require an archive compatibility decision.

## Verification

Race-enabled API tests and PostgreSQL database/realtime integration suites pass. New cases cover passed-out and 13-trick boards, accepted/rejected claims, undo after scoring, exact restart recovery in auction/play, active event gaps, participant-only archive access, unfinished expiry, repeated compaction, and rollback on validation/insert/readback/outcome failures. Migration 00006 was tested down/up on an isolated PostgreSQL 17 database; sqlc generation, migration validation, vet, and lint pass.
