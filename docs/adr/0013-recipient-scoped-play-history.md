# ADR-013: Recipient-Scoped Play History

- Status: Accepted
- Date: 4 September 2026
- Decision owners: Product/Engineering
- Implements: OD-20, ENG-01
- Related: ADR-002, ADR-003, ADR-009

## Context

The private bridge state already persists every completed trick and hydrates it through the versioned table snapshot. The recipient projector currently copies that entire collection to every joined participant. Activating the trick-history entry point without narrowing this payload would make a frontend visibility check the security boundary and would expose more play sequence than the approved product policy.

The client also uses the length of `completedTricks` for gameplay motion and claim limits. Once the projected collection is recipient-scoped, that length no longer represents authoritative progress for non-Dummy viewers.

## Decision

- `bridge.State.CompletedTricks` remains the complete authoritative history and continues to be persisted in the existing private snapshot. No duplicate history store or schema migration is introduced.
- Recipient projection adds `completedTrickCount`, an integer from 0 through 13 that represents authoritative board progress without exposing additional cards.
- A participant whose current seat is the contract Dummy receives completed tricks 1 through the latest completed trick.
- Declarer, both defenders, and any joined unseated participant receive no completed trick before trick 1, then exactly the latest completed trick only.
- The same entitlement applies during play, reconnect/resume, board-scored state, and table lifecycle transitions. Full-deal visibility after scoring remains a separate existing permission and does not broaden ordered play history.
- Bots are seat occupants without a session and never receive a projection.
- The web client renders only the projected collection. It uses `completedTrickCount` for progress, motion boundaries, and remaining-trick calculations; it never reconstructs unauthorized earlier tricks from other payloads.
- History cards, raw projection payloads, and participant identity are not added to logs, traces, or metric labels. Existing bounded projection failure telemetry remains the operational signal.

## Consequences

Positive:

- Dummy can review the complete play sequence available so far.
- Other participants receive only the latest completed trick in raw snapshot and event frames.
- Refresh and reconnect reproduce the same entitlement from authoritative state.
- Existing durable snapshot compatibility is preserved because private state is unchanged.

Trade-offs:

- `completedTricks.length` is no longer a board-progress counter for every viewer.
- Older web clients that assume full projected history must be deployed together with the server projection change.
- A scored full deal exposes card ownership but not the ordered sequence of earlier tricks to non-Dummy viewers.

## Validation

- Projector tests cover declarer, Dummy, both defenders, 0, 1, multiple, and 13 completed tricks.
- Defensive-copy tests prove projected history cannot mutate private authoritative state.
- Reconnect/integration tests inspect snapshot and event frames, not only the DOM.
- Web tests cover progress/motion using `completedTrickCount`, Dummy full history, non-Dummy latest-only history, keyboard close, outside close, and narrow-viewport scrolling.
- Raw-frame assertions fail if a non-Dummy payload contains more than the authorized latest completed trick.
