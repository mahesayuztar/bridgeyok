# ADR 0020 — Internal Team Match

Date: 14 September 2026
Status: accepted; domain, PostgreSQL, authenticated API/actor integration and minimum UI implemented

## Decision

Use the existing Go application boundary and PostgreSQL authority. Exactly two private rooms are created for one match. Eight distinct human participants occupy the eight fixed positions; the owner occupies one position. Team A occupies North/South in the open room and East/West in the closed room. Team B has the inverse orientation. Lineup changes while waiting clear all readiness; all eight participants must be ready to start. Active lineups are fixed in the initial implementation; reconnect retains identity and room. Owner reassignment/substitution policy must be added explicitly before enabling it.

A match contains 1–32 boards numbered from one. Allocate board identities before start. Generate each deal once, with its source provenance and canonical metadata. Generate and validate the entire set before moving to active; persist it atomically before starting either room. Room-specific engine state references the shared match-board identity; existing table-board record identities remain distinct. Never reuse a table board ID across two table records without first updating existing ownership constraints.

Each room plays the same ordered board set independently. Finishing one room does not advance or block the other room. A room may progress only after its own previous result is finalized. Match tables must not accept casual next-board generation, room switching, or unrestricted lineup changes. Existing bot and chat features must not implicitly enable cross-room membership or communications through the match API.

Only finalized, authoritative table results enter match comparison. The application must seal the table result against undo before recording it. The score difference from Team A's perspective is `open.scoreNS - closed.scoreNS`, converted using WBF Law 78B. Team B's signed net is its inverse; no VP conversion or tie-breaker is added. Identical result retries are no-ops; conflicting redelivery is rejected. Per-board IMPs and totals derive from the unique paired results, so delivery count cannot change totals.

Reference: [WBF Laws, Law 78B, printed page 58](https://www.worldbridge.org/wp-content/uploads/2020/02/2017LawsofDuplicateBridge-gender_neutral_1.pdf).

## Privacy

The domain's private snapshot and next-board output contain secret deals and are exclusively for persistence/trusted table orchestration. They must never be serialized as REST, WebSocket, telemetry, or client error payloads. The public match projection returns only the participant's own room assignment, team, and aggregate readiness/progress. Detailed comparisons and totals are withheld until the entire match completes. Nonparticipants are rejected. Waiting-owner setup needs a separate authorized application representation before a lineup exists.

Room membership, replay, DDS, board history, completed-deal projection, and social/table participant APIs require a match-aware authorization audit. Existing casual-table completion entitlement is insufficient when the other room has not played a shared board. Domain projection tests do not establish security of those unmodified routes.

## Persistence and recovery implementation contract

Migration 00010 adds normalized match/table bindings, fixed assignments, immutable shared sources, and results with unique `(match_id, board_id, room)` constraints. Match result rows reference a permanent board archive. Closing a room's board through next-board/finish archives and validates it, collects the authoritative result, inserts any completed comparison, updates the match revision/status/total, deletes compacted events, and stores the command outcome in the same transaction. Scored-but-undoable boards do not enter comparison.

`CreateMatch` is a trusted repository operation accepting a validated waiting domain snapshot and two existing waiting tables with exactly eight matching human seats. Its assignment `ParticipantID` values are stable guest/session identity IDs, not room-local `table_participants.id`. The caller-facing lifecycle service must establish owner/participant authorization and explicit match consent before exposing this operation. Configuration hashes distinguish identical create retries from conflicting reuse of a match ID. Ready changes use the existing revision/request-fenced table command path and update match readiness atomically.

`StartMatch` checks owner and expected match revision, generates the complete set, then commits both first-board engine snapshots and their distinct room-board IDs together. Once started, retries return recovered match state without regenerating or republishing initial events. The service must route the returned committed table batches through existing actor/realtime publication and recover missed delivery through snapshots; the authenticated start endpoint refreshes both actors after commit.

Lock order is sorted table row(s), then match row. Final result writers from the two rooms serialize on the match row. Readers use a repeatable-read transaction, reconstruct through domain validation, and compare persisted IMP rows and total against recalculation. No source/score input from a client is used to collect results. PostgreSQL integration covers concurrent start/finalization, retry, reopened repository recovery, and transaction failure rollback.

The mutable domain object is owned by one serialized application operation. Work on a detached candidate and publish only after commit; it is not concurrently safe on its own. No second actor, Redis, or cross-instance mechanism is introduced.

## Delivery sequence

1. Domain state machine, WBF IMP boundaries, validated private snapshot recovery, and recipient projection.
2. PostgreSQL schema/repositories, atomic shared-board start, sealed result collection, restart/concurrent delivery tests.
3. Authenticated lifecycle API and existing table actor integration, match-aware privacy enforcement across all board routes.
4. Minimal Team Match UI and eight-client/browser validation.
5. Capacity/restart/failure/security drills and closed-beta pilot, retaining the pending independent WBF review and single-instance hosting release gates.

Steps 1–4 are implemented as of 15 September 2026; current verification evidence is recorded in the runbook. Existing table command persistence now selects shared next boards, seals final results, and rejects casual start/lineup/early-finish/extra-board mutations for bound rooms. REST join rejects other-room/new participants and leave cannot change the fixed lineup. Match rooms are excluded from casual inactivity expiry pending a match-level recovery/cancellation policy.

Replay, completed archive, and DDS query authorization additionally require the whole match to be complete, while retaining original room membership checks. The table projection carries match identity, board limit, and completion entitlement for client controls. Production deployment remains pending. Runbook and verification: `docs/operations/phase5-postgres.md`.


## Authenticated lifecycle and UI

`POST /v1/matches` accepts eight registered user IDs, fixed room/seat assignments and 1–32 boards. It creates two fresh rooms atomically; existing casual tables are never captured. Every participant, including the creator, starts unready and explicitly consents. Sorted account row locks serialize request deduplication and a maximum of four pending/active matches per invited account. Reusing an owner/request ID with changed input is rejected. The UI fixes the creator at Open North and lets them choose the other seven seats using existing account search.

`GET /v1/matches` lists only the participant's latest twenty matches. `GET /v1/matches/{id}` exposes their own assignment and public progress. Ready uses the existing table actor command and expected table revision; start uses the match revision and owner authorization. Any assigned participant may cancel a waiting match; cancellation atomically closes both rooms. Active cancellation, substitutions, owner succession, and inactivity expiry remain unavailable.

After committed start/ready/cancel, the service refreshes both existing table actors and publishes recipient-scoped snapshots. A bounded context independent of HTTP cancellation attempts both rooms. Failed publication returns durable success with `syncPending`; retry/reconnect refreshes current state without regenerating boards or replaying stale start events. Detailed comparisons remain withheld until completion. The UI retains a create request identity for unchanged retries and rejects late poll responses across mutations.

Migration 00011 adds cancellation status and durable create request deduplication. The authenticated web proxy forwards only allowlisted match paths with same-origin mutation checks. Team Match routes reuse account guards, account search, server capabilities, existing table gameplay, and design tokens. Returning to the match keeps the fixed seat. DDS and replay remain unavailable during the match; replay entitlement is refreshed on table re-entry after completion.
