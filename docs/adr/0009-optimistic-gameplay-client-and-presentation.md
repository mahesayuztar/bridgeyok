# ADR-009: Optimistic Gameplay Client and Sequenced Presentation

- Status: Accepted
- Date: 1 September 2026
- Decision owners: Product/Engineering
- Implements: OD-18
- Related: ADR-002, ADR-003, ADR-007, ADR-008

## Context

The current React table applies a recipient-specific projection only after a committed WebSocket event. A legal local bid or card therefore appears delayed by database and network latency. `useTableSession` records only `request_id → command name`, ignores accepted ACK envelopes, and `table-state` clears every pending command when any event/snapshot arrives. `bridge-table.tsx` also combines most gameplay domains and has no presentation queue, drag controller, or audio transition boundary.

The authoritative Go engine, table actor, PostgreSQL transaction, revision/sequence fencing, idempotent request ID, controller epoch, and recipient projector are already the correct trust boundary. Reliability requires immediate local feedback without weakening those boundaries or coupling game legality to animation timing.

## Decision A — Server authoritative, client optimistic

PostgreSQL-backed Go table state remains authoritative. The browser may immediately project a locally initiated call or card play only when it is legal in the current projected capability state.

An optimistic operation records at least:

- request ID and command payload;
- authoritative base revision/sequence;
- expected logical projection effect;
- lifecycle state needed for ACK, event, rejection, conflict, or reconnect.

The client derives visible gameplay from the authoritative projection plus ordered pending optimistic operations. Accepted ACK is command receipt; recipient event/snapshot is authoritative state. Rejection removes the affected operation and rolls back/rebases against the latest valid authoritative projection. Unrelated confirmed events must survive that rebase. Snapshot/resume remains the ultimate recovery path, and arbitrary stale intent is never queued or automatically retried offline.

## Decision B — Logical state and presentation state are separate

Client state has three boundaries:

1. authoritative recipient projection received from server;
2. optimistic logical operations projected over that base;
3. presentation events that visualize movement and pacing.

Authoritative sequence/revision processing and local logical projection do not wait for animation. A central presentation queue may visually replay local/remote card movement, completed trick pause, winner indication, and collection while newer logical events are already known. Components do not own independent gameplay timers.

The current normal gameplay animation is interruptible only by clicking/tapping the board. That action settles the active presentation item to its final visual state and continues queue processing. Navbar, dialog, and clicks outside the board do not implicitly skip it. Reduced-motion still communicates origin/result with minimal duration.

## Decision C — One gameplay command path

Click, tap, keyboard activation where applicable, and Pointer Events drag/drop converge on one canonical client action for playing a card. That action performs capability validation, creates one optimistic operation, and emits one `game.play_card` command. Drag/drop does not duplicate bridge rules or use a separate mutation path.

The entire valid board surface is the drop pool for a playable card. Pointer cancellation or invalid release restores presentation without emitting a command.

## Decision D — Known illegal actions are prevented

The current recipient projection supplies legal calls, turn/role/phase, playable own/dummy cards, consensus state, and undo capability. Controls derived as illegal/unavailable are disabled or absent and do not emit a WebSocket command. Predictable bridge-rule invalidity is not normally presented as a toast.

The server remains final validator. A rejection for an action that the client believed legal is treated as stale/conflicting/corrupt projection evidence and reconciled. Network, authentication, revision, controller, malformed projection, and persistence failures remain visible system errors.

## Decision E — Canonical card primitive and stable geometry

Own hand, dummy hand, and current trick use one canonical visual card primitive. Context changes interaction, orientation, scale, state, and emphasis through variants/tokens; it does not create separate card renderers.

Board layout defines stable non-overlapping zones and animation anchors for participants, own hand, oriented dummy suits, and trick slots. Established bridge-table/BBO conventions take precedence over novel arrangement. Gameplay spatial continuity is correctness, not decorative polish.

Domain components are extracted at reusable visual, stateful interaction, or bridge-domain boundaries. Trivial wrappers remain local; the architecture does not replace one monolith with meaningless atomic components.

## Decision F — Engine work is gated

ENG-01 Play History, ENG-02 Table Score Sheet, and ENG-03 Bot Consensus Behavior may not begin until UX-01 through UX-14 all pass GATE UX-G1 in `apps/web/PLAN.md`.

A frontend-discovered engine limitation is documented as a dependency. An exception before UX-G1 requires evidence that the frontend contract is literally impossible, the smallest compatible engine change, an ADR amendment, and explicit roadmap approval. Convenience is not an exception.

ADR-008's current rule that bot presence disables claim/undo remains implemented until ENG-03 intentionally supersedes it after the gate. ENG-01 must also resolve the current broad `CompletedTricks` recipient projection against the approved history information policy. ENG-02 remains blocked until “IMP score sheet” has an explicit comparison source and pair/session lifecycle.

## Consequences

Positive:

- local legal action acknowledges immediately while server authority, idempotency, and hidden-information projection remain intact;
- animation can preserve origin and trick pacing without blocking game state;
- all card inputs share legality and exactly-once command behavior;
- card appearance and board geometry stop drifting by context;
- post-gate engine work cannot destabilize the frontend refactor opportunistically.

Trade-offs:

- the client needs a tested optimistic operation model and semantic presentation queue;
- ACK/event correlation may require a backward-compatible protocol metadata addition after contract tests;
- tests must control network timing and browser geometry rather than relying only on snapshots or compilation;
- presentation may temporarily lag logical state, so accessibility/status output must describe the projected current state without announcing duplicate events.

## Alternatives rejected

| Alternative | Reason rejected |
|---|---|
| Wait for authoritative event before visual feedback | Correct but perceptibly frozen under normal latency. |
| Mutate the authoritative client projection in place | Makes rollback/rebase and snapshot recovery ambiguous. |
| Delay logical state until animation finishes | Lets timing control legality/order and risks dropped remote events. |
| Separate click and drag rule paths | Duplicates legality and enables double commands. |
| Toast every illegal action | Allows predictable invalid input and conflates rules with infrastructure failure. |
| Separate own/dummy/trick card renderers | Creates visual and accessibility drift. |
| Start history/score/bot engine changes during extraction | Expands regression scope before the frontend contract is stable. |

## Validation

- Reducer/model tests cover delayed ACK, event/ACK in either order, rejection, conflict, unrelated event rebase, duplicate delivery, reconnect, and snapshot replacement.
- Raw WebSocket tests prove known-invalid actions emit no command and optimistic valid actions emit exactly one request ID.
- Presentation queue tests cover remote bursts, trick completion, board-only skip, reduced motion, reconnect, and unmount cleanup.
- Pointer tests cover desktop/touch, capture/cancel/scroll, invalid drop, and exactly-once convergence with click play.
- Browser screenshots and bounding boxes cover auction/play, dummy top/left/right, completed trick, tablet orientations, common mobile, and 320–400 px widths.
- Recipient payload inspection protects hidden cards/history independently of the DOM.
- UX-G1 evidence ledger is complete before ENG-01/02/03 begins.
