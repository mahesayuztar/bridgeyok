# ADR 0020 — Internal Team Match

Date: 14 September 2026
Status: accepted for implementation; domain foundation implemented, integration pending

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

Next implementation slice must add match/table bindings, immutable shared boards, fixed assignments, and results with unique `(match_id, board_id, room)` constraints. Serialize mutations with a PostgreSQL row lock and commit both table result sealing and match result/comparison in one transaction. Persist final status/results and recover by validated hydration. Do not treat the domain's in-memory idempotency or JSON round-trip tests as database exactly-once evidence.

The mutable domain object is owned by one serialized application operation. Work on a detached candidate and publish only after commit; it is not concurrently safe on its own. No second actor, Redis, or cross-instance mechanism is introduced.

## Delivery sequence

1. Domain state machine, WBF IMP boundaries, validated private snapshot recovery, and recipient projection.
2. PostgreSQL schema/repositories, atomic shared-board start, sealed result collection, restart/concurrent delivery tests.
3. Authenticated lifecycle API and existing table actor integration, match-aware privacy enforcement across all board routes.
4. Minimal Team Match UI and eight-client/browser validation.
5. Capacity/restart/failure/security drills and closed-beta pilot, retaining the pending independent WBF review and single-instance hosting release gates.

Only step 1 is implemented by this checkpoint. No Team Match endpoint, UI, migration, or deployment is enabled yet.
