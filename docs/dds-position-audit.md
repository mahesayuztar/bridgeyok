# DDS position audit — 8 September 2026

## Baseline findings

- `analysis.Service.Analyze` called `CompletedAnalysisBoard`, which rejected unfinished boards, and passed the original deal to `DDS.Solve`.
- `tools/dds/main.cpp` called only `CalcDDtable` and `DealerParBin`. It did not call `SolveBoard` or supply remaining holdings/current-trick cards. The existing output was a 20-entry deal table and par, not predictions for individual legal cards.
- `board-replay.ts` advanced by completed trick and removed four cards at a time. Partial tricks and opening-lead transitions could not be selected.
- `BoardReplayModal` used `showModal()` on all viewports; its mobile CSS allowed the entire viewport. The live table was inert while replay was open.
- No per-card DDS UI/request state existed. Replay loading already had an abort fence and board identity validation; position analysis extends this pattern.

## Implementation boundary

`GET /v1/boards/{boardId}/analysis` retains its original deal-analysis behavior without query parameters. `positionKey` selects position analysis. For live play, it is the exact table revision; `step` selects the number of played cards in a completed historical board and makes `positionKey` a response correlation identity.

The repository retains membership checks, validates the private snapshot, and uses the existing recipient projector to prevent a position request from revealing a hidden opponent hand. Live predictions are available for the viewer's active hand or an already revealed dummy. No full private deal is serialized in the position response.

Historical positions reuse `CompletedBoardReplay` and replay recorded calls/cards through `bridge.Decide`. No rule evaluation or trick-winner solver is added to React. The frontend's existing replay view model filters the original hands using the recorded card prefix, consumes recorded winners, and keeps the displayed just-completed trick separate from the next logical current trick.

DDS uses `SolveBoard(position, -1, 3, 1, ...)`, supplies current-trick leader/cards and remaining holdings, expands equivalent ranks, then validates the complete output against engine `LegalCards`. The numbers represent **total tricks for the partnership currently on turn**, adding tricks already won. Timeout, bounded output, worker capacity, and subprocess cancellation remain in the existing DDS boundary. See the [pinned DDS API description](https://github.com/dds-bridge/dds/blob/8d75755c8df81999557758c9757514edb94017bc/doc/dll-description.md).

The shared position hook fences by board, revision/history cursor and effect cancellation. A new position hides previous values during render, before the new request completes. Optimistic live commands suppress analysis until the authoritative revision arrives. Legal analysis cards are independent of permission to submit a play; e.g. a visible dummy can be annotated without becoming controllable.

`PlayingCard` remains the canonical renderer for loading indicators and results. Illegal cards receive neither. Spinner motion is disabled under reduced motion. Replay uses the historical board ID/cursor rather than the live table revision.

## Scoped React review coverage

Mode A: feature scope only. Groups below include `bridge-table`, replay view model/modal, `PlayingCard`/`BridgeHand`, `usePositionAnalysis`, session request wiring, and status/score-sheet wiring. This is not a repository-wide React audit.

| Category | Replay/state | Card UI | Analysis hook/session | Status wiring |
| --- | --- | --- | --- | --- |
| 1 Concurrent rendering | clean: frame is derived | clean | clean: render-time key fence | clean |
| 2 Server components | n/a: interactive boundary | n/a | n/a | n/a |
| 3 Actions/forms | n/a | clean: existing play path | n/a | clean |
| 4 Data fetching | fixed: historical position selection | n/a | added abort + identity fence | clean |
| 5 State management | fixed: per-card cursor/partial trick | clean | separate request state | clean |
| 6 Memoization/performance | clean | clean | stable authenticated loader | clean |
| 7 Effects/events | existing replay cancellation retained | clean | cleanup discards late results | clean |
| 8 Component patterns | reuse table/card boundaries | canonical renderer retained | shared meaningful hook | no new wrapper |
| 9 Codebase hygiene | reuse replay repository and engine | reuse legal-hand helper | reuse DDS/API domain | reuse navbar space |

The parallel dialog drag/layout work belongs to another session and is not part of this audit's implementation ownership.

## Validation

- Unit reconstruction checks cover every prefix of a 52-card engine game; frontend checks cover rewind, partial tricks, opening lead, result, passed-out and claimed boards.
- Pinned DDS binary tests cover positions 0–4 and 45–51, legal-card completeness, equivalent cards, final-card totals, and consistency with the next position's optimum. Original deal golden fixtures remain green.
- HTTP tests cover authentication, membership errors, revision mismatch, cursor bounds, identity echo, and no-store headers.
- Database integration checks cover active snapshot selection, stale revision rejection, outsider rejection, and completed-board historical reconstruction. Passed-out and claim scenarios pass against the configured database. The full 52-card persistence scenario hit the existing 45-second database test context deadline during CARD_PLAYED insertion; this is not a solver assertion failure.
- Browser component tests exercise real React components with deliberately uncancellable deferred DDS promises for history changes and live-card actions. The same tests check partial tricks and mobile table interaction.

## Verification follow-up — 9 September 2026

The focused browser matrix passes at 1440×900, 768×1024, 390×844 and 320×700. It verifies uncancellable late responses after a live-card action and history navigation, follow-suit filtering, DD off/on loading transitions, replay card counts, and pointer interaction behind the mobile panel. Endpoint navigation retains focus when its button becomes disabled. Closing replay delegates focus restoration to Score History, avoiding a second cleanup handler overriding the correct trigger.

Frontend typecheck and 53 unit tests pass. Focused ESLint checks pass; the wider web lint reports the two pre-existing `aria-description` warnings. Go vet, golangci-lint, race-enabled analysis/HTTP/database unit tests, and pinned DDS integration fixtures pass. Database privacy checks reject another participant's hidden active hand and a stale revision.

The full four-player Playwright completed-board replay scenario also passes (1.7 minutes): play all 52 cards, open the persisted replay, check desktop/mobile geometry, inject delayed DDS responses, navigate every card through the result, close with Escape, restore the originating score-row focus, and reopen the board.
