# ADR-014: Table Score Sheet Semantics

- Status: Accepted
- Date: 5 September 2026
- Decision owners: Product/Engineering
- Implements: OD-21, ENG-02
- Related: ADR-002, ADR-003, ADR-008, ADR-009

## Context

One BridgeYok table can play multiple boards, but participants currently lose the earlier results when the aggregate advances to the next board. The roadmap called the missing surface an "IMP pair-oriented" score sheet even though a single table produces only one duplicate result for each board. An IMP value requires a comparison result with the same board and a defined orientation; that source exists only in the future two-table Team Match.

Participants may change seats between boards, and an active-board occupant may be replaced by a bot. A cumulative score therefore cannot safely infer pair identity from the table's current seats.

## Decision

- ENG-02 is named **Table Score Sheet** and displays raw duplicate points. The UI uses "Skor meja" and never labels a one-table value as IMP.
- The score-sheet session is exactly one `table_id`, from table creation until its terminal state. It contains every scored or passed-out board in ascending board-number order.
- Each row is identified by immutable `board_id` and `board_number`, and traces to the persisted board result whose canonical signed value is `score_ns`. EW's board value is derived as `-score_ns`; it is not stored independently.
- A pair identity is the canonical, order-independent combination of the two occupant IDs assigned to one partnership when the board starts. The participant display names and seat mapping are captured with that board lineup.
- Swapping N/S positions, swapping E/W positions, or moving the same pair from NS to EW on a later board preserves pair identity. A pair's cumulative table score adds `score_ns` when the pair played NS and `-score_ns` when it played EW.
- Replacing an occupant during an active board is a substitution. It does not rewrite the board-start pair attribution. A later board captures the then-current lineup as a new immutable attribution.
- A passed-out board contributes zero and remains visible. Positive, negative, and zero totals are valid; equal totals are not converted into a separate tie score.
- IMP conversion is absent from this feature. It remains owned by Phase 5 Team Match, which supplies the paired result, shared board identity, comparison orientation, and WBF Law 78B scale.
- The private aggregate snapshot retains the current board lineup and completed score rows for deterministic reconnect projection. Relational board-lineup rows provide a durable audit link from each score row to its board result.
- Only joined table participants receive the score sheet through the existing recipient projection. It contains result and attribution data only, never a deal, hand, credential, invite, or session identifier.
- Score-sheet values, participant IDs, and display names are not metric labels and are not added to logs or traces. Existing command persistence/projection errors remain the operational signals.

## Consequences

Positive:

- The navbar can show a useful multi-board ledger before Team Match exists without inventing a comparison baseline.
- Pair totals remain correct when a pair changes orientation between boards.
- Refresh, reconnect, and API restart reproduce the same rows from authoritative state.
- Phase 5 can add IMP comparison without redefining or migrating one-table duplicate scores.

Trade-offs:

- The table total is a casual session total, not a tournament ranking, matchpoint result, rubber score, or IMP result.
- A mid-board substitute is visible in current occupancy but does not become part of that board's score identity.
- Older snapshots created before ENG-02 have no historical lineup facts to reconstruct safely; they remain readable with an empty sheet, and only boards started under this contract enter the ledger.

## Validation

- Pure table tests cover stable pair IDs, seat-order swaps, NS/EW orientation, positive, negative, zero, and multi-pair totals.
- Aggregate tests cover score append exactly once, passed out, undo after score, re-score, next-board lineup capture, and active-board replacement.
- PostgreSQL integration tests verify board result and lineup persistence in one transaction, duplicate command delivery, and restart hydration.
- Projector and contract tests prove the score sheet is participant-only, contains no session or hidden-card data, and has deterministic ordering.
- Web unit and Playwright tests cover the navbar action, empty/current/multi-board rows, pair totals, refresh/reconnect, keyboard dismissal, focus return, and scrolling at 320 px through desktop.
