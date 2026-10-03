# History UI Audit — Brief Utama, Cache, dan Work Plan

Status dokumen: `READY FOR AGENT EXECUTION`

Tanggal penyusunan: 2026-10-04

Scope: `apps/web` — `ScoreSheet`, `BoardReplayModal`, History workspace, replay table geometry, responsive behavior, accessibility, dan styling terkait.

Sumber brief: audit repository yang diberikan user. Audit awal bersifat read-only dan tidak mengubah file.

## Brief utama untuk agent lain

Kerjakan audit/perbaikan UI History secara bertahap berdasarkan work items di dokumen ini. Pertahankan server-authoritative projection, `GameSessionProvider`/`useTableSession` sebagai pemilik sesi tunggal, native Popover/Dialog yang sudah digunakan, primitive kartu yang sama, dan batas replay read-only. Jangan membuat history backend baru, localStorage archive palsu, scoring client-side, legality engine kedua, atau state owner baru.

Sebelum mengubah file, baca `AGENTS.md`, `PLAN.md`, `apps/web/PLAN.md`, `apps/web/app/globals.css`, dan bagian work item yang dikerjakan. Periksa `git status` dahulu. Worktree mungkin sudah mempunyai perubahan pengguna; jangan reset, checkout, stash, atau menimpa perubahan yang tidak terkait.

Setiap work item yang selesai WAJIB dicache sebelum agent berhenti:

1. Tambahkan atau perbarui file cache item pada `docs/agent-cache/history-ui-audit/` dengan status, tanggal, agent, file yang berubah, keputusan, verifikasi, screenshot/viewport, isu tersisa, dan commit hash bila sudah commit.
2. Update tabel status di dokumen ini secara singkat.
3. Jangan menyatakan item `done` bila verifikasi yang diwajibkan belum dilakukan; gunakan `blocked`, `partial`, atau `needs-follow-up`.
4. Cache adalah handoff antar-agent, bukan tempat menyembunyikan kegagalan test.

Template cache wajib:

```md
# Cache WI-XX — <judul>

- Status: `done` | `partial` | `blocked` | `needs-follow-up`
- Agent:
- Completed at:
- Depends on:
- Files changed:
- Decisions:
- Verification commands:
- Browser evidence:
- Known failures or unrelated failures:
- Commit(s):
- Next agent:
```

Commit implementasi, bila diminta atau dilakukan sesuai workflow repository, hanya boleh memakai format berikut:

```text
+ action: concise lowercase description
+ action: second concise description if needed
```

Tidak boleh ada subject line tambahan, attribution trailer, atau `Co-authored-by`.

## Target outcome

History dan replay harus terasa sebagai bagian dari bridge client yang ringkas dan dapat diprediksi:

- `ScoreSheet` tetap menjadi entry point hasil board dari active table.
- `BoardReplayModal` tetap menjadi read-only replay yang dipakai active table dan `/history`.
- Replay tidak mengirim command dan tidak mengubah projection authoritative.
- Tabel score dapat dibaca pada desktop, tablet, coarse pointer, dan lebar sekitar 320–400 px tanpa horizontal overflow yang tidak disengaja.
- Replay mempertahankan geometri N/E/S/W, dummy, trick, own hand, auction, result, dan navigasi step.
- Modal/popover memiliki close, Escape, light-dismiss/native behavior, focus restoration, loading, error, retry, dan status yang dapat diakses.
- Scrolling memiliki satu boundary yang jelas; nested scroll hanya dipertahankan jika benar-benar diperlukan oleh auction/replay geometry.
- Style memakai token `globals.css`, tanpa gradient, tanpa generic dashboard treatment, tanpa modal library/dependency baru, dan tanpa fake window controls.
- Tidak ada perubahan protocol, engine, persistence, authorization, hidden-hand projection, atau session ownership.

## Current architecture dan source of truth

### Active table flow

```text
AppShell
→ BridgeTable
→ ActiveTableStatusBar / WaitingTableStatusBar
→ ScoreSheet
→ score-sheet-popover
→ BoardReplayModal
→ TableSurface
→ CompletedDeal
→ BridgeHand
→ PlayingCard
```

### History page flow

```text
HistoryPage
→ HistoryWorkspace
→ HistoryBoardList
→ HistoryBoardRow
→ HistoryBoardActions
→ BoardReplayModal
→ TableSurface
```

### Data and authority boundary

```text
server snapshot/event
→ reduceTableState()
→ TableClientState.table
→ projectedTableState()
→ projectOptimisticTable()
→ LiveTableProjection
→ ScoreSheet / HistoryWorkspace
```

`scoreSheet` sudah divalidasi melalui `normalizeScoreSheetEntry()` di `apps/web/app/table-projection.ts`. Replay diambil melalui `loadBoardReplay()` dari `use-table-session.ts`, kemudian dinormalisasi/diubah oleh `replayFrame()`. DDS analysis tetap melalui `loadPositionAnalysis()`/`usePositionAnalysis()` dan hanya presentation data.

### Relevant files

| Area | File | Peran |
| --- | --- | --- |
| active score entry | `apps/web/app/table/score-sheet.tsx` | native Popover, score rows, selection, focus restoration, mount replay |
| replay behavior | `apps/web/app/table/board-replay-modal.tsx` | native Dialog, loading/error/retry, DD, step navigation, geometry scaling, replay read-only projection |
| replay model | `apps/web/app/board-replay.ts` | replay normalization and `replayFrame()` |
| history page | `apps/web/app/history-workspace.tsx` | authorized history feed, search, labels, selection, replay entry/table adapter |
| route | `apps/web/app/(app)/history/page.tsx` | History page boundary |
| session boundary | `apps/web/app/use-table-session.ts` | `loadBoardReplay`, `loadPositionAnalysis`, session ownership |
| projection | `apps/web/app/table-projection.ts`, `apps/web/app/table-state.ts` | authoritative/optimistic projection and score validation |
| table geometry | `apps/web/app/table/table-surface.tsx` | N/E/S/W table layout and play zone |
| cards | `apps/web/app/table/completed-deal.tsx`, `bridge-hand.tsx`, `playing-card.tsx` | canonical hand/dummy/trick card rendering |
| replay children | `apps/web/app/table/auction-controls.tsx`, `current-trick.tsx`, `board-result.tsx` | auction, trick, result states |
| styles | `apps/web/app/globals.css` | source of truth for tokens and all relevant selectors |
| tests | `apps/web/app/board-replay.test.mjs`, `table-projection.test.mjs`, `apps/web/e2e/` | existing contract/projection/E2E evidence |

## Guardrails

- Do not alter Go engine, API contracts, SQL, WebSocket envelopes, authorization, replay response shape, or session provider for a visual task.
- Do not create a second History component for mobile and desktop unless an actual independent behavior boundary is proven.
- Do not replace native Popover/Dialog with Radix, Headless UI, React Aria, MUI, Framer Motion, Zustand, Redux, or another dependency.
- Do not use `window.innerWidth` alone as a semantic breakpoint for gameplay legality or state; CSS/container geometry should own presentation where possible.
- Do not use `display: none` to hide important board state on mobile.
- Do not turn replay into an editable table; `canSendCommand={() => false}` and no-op `onCommand` must remain or be replaced by an equally explicit read-only boundary.
- Do not add arbitrary colors when a `:root` token exists. Do not add any gradient.
- Use `_index` for every index variable if one is needed.
- Avoid explanatory comments in function bodies. Add JSDoc only when intent is not obvious.
- Preserve current uncommitted work. At the time this plan was written, `git status` reported changes in account proxy, `globals.css`, auction controls, chat/table E2E specs, and `next-env.d.ts`; inspect before touching any of them.

## Dependency map

```text
WI-00 repository baseline/reference gate
  ├── WI-01 interaction/accessibility contract audit
  ├── WI-02 CSS cascade/token and geometry audit
  ├── WI-03 data/privacy/replay boundary regression audit
  └── WI-04 browser evidence harness

WI-01 + WI-02 + WI-03
  ├── WI-05 ScoreSheet interaction and semantics
  ├── WI-06 BoardReplayModal lifecycle and navigation
  ├── WI-07 responsive replay/table geometry
  └── WI-08 HistoryWorkspace master-detail consistency

WI-05 + WI-06 + WI-07 + WI-08
  └── WI-09 integrated browser verification and regression hardening

WI-09
  └── WI-10 final cache, review, and focused commits
```

### Parallel work

After `WI-00`, these can be investigated in parallel because they have separate primary concerns:

- `WI-01` interaction, semantics, keyboard, focus, and native lifecycle.
- `WI-02` CSS cascade, tokens, overflow, breakpoints, and geometry.
- `WI-03` projection, privacy, read-only replay, and data contract.
- `WI-04` browser test/evidence setup.

After the audit items, implementation can be parallelized only by file ownership:

- `WI-05`: `score-sheet.tsx` plus narrowly related score selectors/tests.
- `WI-06`: `board-replay-modal.tsx` plus replay lifecycle tests.
- `WI-07`: `globals.css` and geometry tests; coordinate with any concurrent style edit.
- `WI-08`: `history-workspace.tsx` and history selectors/tests.

Do not let two agents edit `globals.css` concurrently. Do not let two agents edit `board-replay-modal.tsx` or `score-sheet.tsx` concurrently. If a work item needs another item’s changed API, wait for that dependency rather than inventing an adapter.

## Work item status

| ID | Status | Depends on | Parallel group | Primary owner/files |
| --- | --- | --- | --- | --- |
| WI-00 | `done` | none | gate | repository/docs/reference |
| WI-01 | `done` | WI-00 | A | `score-sheet.tsx`, `board-replay-modal.tsx` |
| WI-02 | `partial` | WI-00 | A | `globals.css`, table geometry |
| WI-03 | `done` | WI-00 | A | projection/replay/session files |
| WI-04 | `done` | WI-00 | A | Playwright/E2E evidence |
| WI-05 | `partial` | WI-01, WI-02, WI-03 | B | `score-sheet.tsx` |
| WI-06 | `done` | WI-01, WI-02, WI-03 | B | `board-replay-modal.tsx` |
| WI-07 | `done` | WI-02, WI-04 | B | `globals.css`, `table-surface.tsx` only if required |
| WI-08 | `partial` | WI-01, WI-03 | B | `history-workspace.tsx` |
| WI-09 | `pending` | WI-05–WI-08 | C | E2E, browser evidence, regression fixes |
| WI-10 | `pending` | WI-09 | D | cache/review/commits |

---

## WI-00 — Baseline, scope lock, and reference gate

Status: `done`

### Goal

Establish the exact incumbent behavior and prevent agents from implementing assumptions that conflict with current projection/session architecture.

### Required reads

- `AGENTS.md`, `PLAN.md`, `apps/web/PLAN.md`, `apps/web/AGENTS.md`.
- This document and the provided audit brief.
- `apps/web/app/globals.css`, especially `:root`, score-sheet rules, replay rules, table geometry, and responsive rules.
- `score-sheet.tsx`, `board-replay-modal.tsx`, `history-workspace.tsx`, `board-replay.ts`, `table-projection.ts`, `table-state.ts`, `use-table-session.ts`.
- Existing references: `docs/screenshot_bbo.png`, `docs/screenshot_bbo_2.png`, and relevant `docs/product/phase6-evidence/*.png`.

### Baseline checks

```bash
git status --short
git diff --check
rg -n "score-sheet|board-replay|replay-surface|table-surface|history-" apps/web/app/globals.css
rg -n "ScoreSheet|BoardReplayModal|loadBoardReplay|loadPositionAnalysis" apps/web/app
```

Record viewport/reference assumptions, current failures, and all unrelated dirty files in `docs/agent-cache/history-ui-audit/WI-00.md`.

### Done criteria

- Existing behavior and ownership boundaries are written down.
- Reference screenshots are identified; no branded asset copying is permitted.
- Dirty worktree is recorded and protected.
- No implementation begins before this cache exists.

Completion cache: `docs/agent-cache/history-ui-audit/WI-00.md`.

## WI-01 — Interaction, semantics, and accessibility audit

Status: `done`

Depends on: `WI-00`

### Goal

Audit native Popover/Dialog behavior and define the minimum interaction contract before changing JSX or state.

### Inspect

- `ScoreSheet` trigger: `popoverTarget`, `aria-haspopup`, `aria-label`, title label, empty state.
- `score-sheet-popover`: `role="dialog"`, `aria-labelledby`, native Escape/light-dismiss, close button.
- Row selection: row click bubbling, replay button semantics, disabled Team Match rule, trigger focus restoration.
- `BoardReplayModal`: `show()` vs `showModal()`, `cancel`, Escape handler, close callback, portal, retry, DD toggle, arrow navigation, nav focus.
- History trigger: `replayTriggerRef`, selection button, replay button, close focus restoration.
- Status semantics: `role="alert"`, `role="status"`, `aria-busy`, disabled controls, visible focus.
- Pointer drag header and keyboard behavior; ensure dragging never steals interactive child events.

### Decisions to record

- Whether row click should remain a convenience in addition to the actual button, or be changed to one semantic activation path.
- Whether reopening the ScoreSheet after replay close is intended for both active table and history page.
- Which element receives initial focus when replay opens and which receives focus after close.
- Whether mobile non-modal `dialog.show()` is intentional product behavior; do not silently convert it to modal without documenting the effect.
- Exact accessible names for previous/next replay controls; avoid relying on arrows alone.

### Likely files

- `apps/web/app/table/score-sheet.tsx`
- `apps/web/app/table/board-replay-modal.tsx`
- `apps/web/app/history-workspace.tsx`
- tests adjacent to those components and `apps/web/e2e/`

### Done criteria

- Keyboard/focus state machine is documented.
- Any accessibility defect is assigned to a later item with an exact selector/component target.
- No visual change is made solely from an assumption about native dialog behavior.

Completion cache: `docs/agent-cache/history-ui-audit/WI-01.md`.

## WI-02 — CSS cascade, token, overflow, and geometry audit

Status: `done`

Depends on: `WI-00`

### Goal

Make the style work traceable. Resolve duplicated/late cascade rules before adding new selectors.

### Inspect and classify selectors

#### ScoreSheet and popover

- `.score-sheet-trigger`
- `.score-summary`
- `.score-sheet-popover`
- `.score-sheet-popover::backdrop`
- `.score-sheet-header`
- `.score-sheet-header .eyebrow`
- `.score-sheet-header h2`
- `.score-sheet-close`
- `.score-sheet-empty`
- `.score-sheet-table-wrap`
- `.score-sheet-table-wrap:focus-visible`
- `.score-sheet-table`
- `.score-sheet-table caption`
- `.score-sheet-table th`, `.score-sheet-table td`
- `.score-sheet-table thead th`
- `.score-sheet-table tbody tr`, `.score-replay-trigger`

#### Replay dialog and content

- `.board-replay-modal`
- `.board-replay-modal::backdrop`
- `.board-replay-modal .score-sheet-header`
- `.board-replay-modal .score-sheet-header h2`
- `.board-replay-modal .score-sheet-header button`
- `.replay-message`
- `.replay-surface-wrap`
- `.replay-surface-wrap > .table-surface`
- `.replay-surface-wrap .auction-table-wrap`
- `.replay-navigation`
- `.replay-navigation span`, `.replay-navigation button`

#### Shared table geometry and card scale

- `.table-surface`
- `:is(.active-table-client, .replay-surface-wrap)`
- `.bridge-hand`, `.bridge-hand .hand-cards`, `.hand-card-slot`
- `.current-trick` and its positional variants
- `.board-result`
- `.auction-table-wrap`, `.auction-table`
- `.physical-card` and card custom properties

#### History page

- `.history-workspace`
- `.history-section-heading`
- `.history-search-field`
- `.history-board-list`
- `.history-board-row`
- `.history-board-select`
- `.history-board-result`, `.history-board-time`
- `.history-board-label`
- `.history-board-actions`, `.history-replay-button`
- `.history-load-error`, `.history-empty-copy`

#### Container and viewport behavior

- `.active-table-client`, `.play-board-client`, `.gameplay-region`
- breakpoint rules at 64rem, 48rem, 25rem, 22rem and coarse pointer/orientation variants.

### Specific risks to verify

- Earlier `.score-sheet-popover` width/min-width rules are overridden by later rules; identify the final computed result rather than editing the first occurrence.
- `.score-sheet-popover` has viewport max-height but the table wrapper owns the scroll; preserve a single readable scroll boundary.
- `.board-replay-modal` and `.replay-surface-wrap` use `overflow: hidden`, while auction has nested scrolling; test keyboard and touch scrolling.
- Replay surface uses JS transform scaling with `max(1000, window.innerWidth)` and `max(650, width * .56)`; verify no zero/negative scale and no stale height after orientation changes.
- Desktop `showModal()` and mobile/coarse `show()` produce different top-layer/backdrop/focus behavior.
- Do not fix a local replay issue by changing shared `.table-surface` or `.physical-card` in a way that regresses active gameplay.

### Done criteria

- Final computed style/cascade map exists in cache.
- Token reuse and any required new token are justified.
- Each overflow container has an explicit purpose and keyboard/touch behavior.
- Every style change is assigned to a work item and has desktop/mobile acceptance checks.

## WI-03 — Projection, privacy, and replay boundary audit

Status: `ready`

Depends on: `WI-00`

### Goal

Prove UI changes do not weaken authoritative data, privacy, replay authorization, or read-only behavior.

### Verify

- `normalizeScoreSheetEntry()` rejects invalid board IDs, board numbers, result, lineup, ordering, and recipient trick leakage.
- `HistoryWorkspace` uses authorized `GET /v1/history/boards` data and local `table.scoreSheet` fallback only for active session context.
- History navigation is presentation-only: no table GET, socket reconnect, subscribe/resume, takeover, or session-owner change.
- `scoreEntryFromHistory()` does not fabricate hidden hands; replay fetch remains server-authorized through `loadBoardReplay()`.
- `replayTableFor()` is a presentation adapter and must not become a second source of score or legality truth.
- `BoardReplayModal` passes `canSendCommand={() => false}` and no-op command callback to `TableSurface` descendants.
- DD analysis remains optional, pending/failed states remain truthful, and it never changes replay legality.
- Active Team Match rows remain disabled until match completion where required by current contract.

### Files

- `apps/web/app/table-projection.ts`
- `apps/web/app/table-state.ts`
- `apps/web/app/board-replay.ts`
- `apps/web/app/use-table-session.ts`
- `apps/web/app/history-workspace.tsx`
- `apps/web/app/table/board-replay-modal.tsx`

### Done criteria

- A privacy/authority checklist is cached.
- Any required regression test is named before implementation.
- No API/SQL change is proposed unless an actual contract defect is proven and separately approved.

Completion cache: `docs/agent-cache/history-ui-audit/WI-03.md`.

## WI-04 — Browser evidence and regression harness

Status: `ready`

Depends on: `WI-00`

### Goal

Define repeatable evidence for geometry and pointer behavior; compilation alone is not completion.

### Required viewport matrix

| Class | Viewports |
| --- | --- |
| desktop | 1440×900 and 1280×800 |
| tablet/coarse | 768×1024 and 1024×768 |
| mobile portrait | 320×800, 360×800, 390×844, 400×844 |
| mobile landscape | 844×390 or equivalent |

### Required scenarios

1. Empty ScoreSheet.
2. ScoreSheet with several boards; focus table wrapper and sticky header.
3. Open replay from active table; loading, success, error, retry, and close.
4. Replay step 0 auction, mid-trick, final result, previous/next disabled states.
5. DD off, pending, success, failed.
6. Close by button, Escape, native cancel/light-dismiss where applicable, and focus restoration.
7. History workspace selected row, search, label editing, infinite-scroll sentinel, replay open/close.
8. Team Match replay disabled before completion.
9. Pointer coarse and desktop pointer behavior.

### Geometry assertions

- No horizontal document overflow at 320–400 px unless an explicitly labeled table region owns it.
- N/E/S/W positions do not overlap or move unpredictably between auction, play, trick, and result.
- Own hand, dummy, trick, and player zones remain visible and non-overlapping.
- Replay navigation stays reachable and is not hidden behind the scaled surface.
- Header/close/DD controls remain reachable after scaling, orientation change, and viewport resize.
- No fixed-height region clips primary status or controls.

### Evidence

Use existing Playwright configuration and project harness. If the full DB-backed setup is unavailable, record the limitation and use an isolated fixture/component route only when it represents real projection data. Capture screenshots for desktop and mobile before/after relevant work items. Do not call partial harness success full E2E success.

## WI-05 — ScoreSheet interaction and semantics

Status: `pending`

Depends on: `WI-01`, `WI-02`, `WI-03`

### Scope

- Keep `ScoreSheet` as native Popover entry point.
- Make row/replay activation semantically clear and avoid accidental double activation from bubbling.
- Preserve current score calculation and compact labels; do not add client scoring.
- Preserve empty, Team Match-disabled, loading-free projection behavior.
- Improve accessible name/description, focus order, focus restoration, and status text only where audit identifies a defect.
- Ensure selected board lifecycle cannot leave stale selected entry after live table projection changes.

### Likely files

- `apps/web/app/table/score-sheet.tsx`
- focused score-sheet selectors in `apps/web/app/globals.css`
- component/E2E test file selected during WI-01

### Acceptance

- Trigger opens/closes in all supported pointer/keyboard modes.
- A board is opened through one predictable action path.
- Replay close returns focus to the originating trigger and does not create a duplicate popover/replay state.
- Empty state and disabled Team Match state remain truthful.
- No active table navigation/session churn is introduced.

## WI-06 — BoardReplayModal lifecycle and navigation

Status: `done`

Depends on: `WI-01`, `WI-02`, `WI-03`

### Scope

- Preserve native `<dialog>` and portal unless WI-01 proves a native bug that cannot be handled locally.
- Harden open/close lifecycle against repeated effect calls, media changes, unmount, and stale replay requests.
- Keep `AbortController`, retry, invalid replay guard, DD analysis, arrow navigation, and read-only callbacks.
- Make loading/error/retry/result announcements clear without shifting table geometry unnecessarily.
- Ensure previous/next controls expose useful labels and stable disabled behavior.
- Preserve trigger focus restoration for both ScoreSheet and HistoryWorkspace parents.

### Likely files

- `apps/web/app/table/board-replay-modal.tsx`
- `apps/web/app/board-replay.ts` only if step semantics expose a real defect
- `apps/web/app/globals.css` replay selectors
- focused replay tests

### Acceptance

- Replay requests are aborted on close/unmount and stale results cannot paint a newly selected board.
- `BOARD_SCORED`/result validation remains enforced.
- Replay never emits gameplay commands.
- Escape, cancel, close button, retry, keyboard arrows, and navigation buttons remain deterministic.
- DD failure is a truthful status, not a fake result or blocking error.

## WI-07 — Responsive replay and table geometry

Status: `done`

Depends on: `WI-02`, `WI-04`

### Scope

- Normalize relevant repeated CSS rules in `globals.css` only as needed for correctness; do not perform unrelated CSS cleanup.
- Preserve shared table/card geometry for active gameplay while allowing replay-specific containment.
- Make score popover, replay dialog, replay surface, auction scroll, and navigation usable at 320–400 px.
- Verify coarse pointer/mobile landscape behavior and `show()`/`showModal()` implications.
- Prefer existing CSS tokens and scale variables. If JS scaling remains necessary, document its invariant and test resize/orientation.
- Keep result, trick, dummy, and own-hand zones spatially stable.

### Likely selectors/files

- `apps/web/app/globals.css`: all selectors listed in WI-02.
- `apps/web/app/table/board-replay-modal.tsx`: only if resize/lifecycle logic must be corrected.
- `apps/web/app/table/table-surface.tsx`: only if a replay-specific geometry hook is necessary and no CSS-only solution exists.
- card/table children only if a shared geometry defect is proven.

### Acceptance

- No gameplay geometry regression in active table screenshots.
- No clipped primary controls at mobile portrait/landscape sizes.
- Scroll regions are reachable by wheel, touch, keyboard, and assistive technology.
- No unintentional document-level horizontal scroll.
- No gradient, arbitrary color, fake window controls, or generic dashboard card treatment.

## WI-08 — HistoryWorkspace consistency and master-detail behavior

Status: `partial`

Depends on: `WI-01`, `WI-03`

### Scope

- Preserve unified authorized board feed, keyset pagination, duplicate filtering, server-side search, labels, and active score-sheet fallback.
- Keep History navigation presentation-only and avoid session ownership changes.
- Ensure selected board remains stable through pagination/search/live match refresh where intended.
- Make replay action, disabled Team Match state, label input, load error, empty result, and loading state coherent.
- Preserve focus restoration to the history replay trigger.
- Avoid introducing a second replay implementation; continue using `BoardReplayModal`.

### Likely files

- `apps/web/app/history-workspace.tsx`
- `apps/web/app/(app)/history/page.tsx` only if route-level semantics require it
- history selectors in `apps/web/app/globals.css`
- history E2E tests

### Acceptance

- Search/pagination/label behavior remains authoritative and deduplicated.
- Selected state is keyboard-visible and does not rely only on color.
- Replay opens from the selected board and closes back to the original action.
- No full table fetch, socket reconnect, or active session mutation occurs merely by viewing history.

## WI-09 — Integrated browser verification and regression hardening

Status: `pending`

Depends on: `WI-05`, `WI-06`, `WI-07`, `WI-08`

### Required checks

```bash
git diff --check
npm --prefix apps/web run lint
npm --prefix apps/web run typecheck
npm --prefix apps/web run test
```

Use the repository's actual scripts if names differ; record exact commands and output. Run focused tests first, then the smallest relevant full suite. Run browser checks at every WI-04 viewport. If the project requires DB-backed Playwright, use the documented environment setup without exposing secrets.

### Regression checklist

- Active table auction/play/score still works.
- Shared `PlayingCard` remains the only card primitive.
- Dummy and trick history privacy projection remains correct.
- Replay remains read-only.
- ScoreSheet does not cause navigation churn.
- History does not create a second session owner.
- Chat sibling/layout is unaffected.
- Existing unrelated worktree changes remain intact.
- No new dependency is present.

### Failure handling

Classify every failure as:

- introduced by this work item;
- pre-existing and reproducible;
- harness/environment/fixture failure;
- unrelated dirty-worktree failure.

Do not report a partial browser run as full pass. Cache each classification in `WI-09.md`.

## WI-10 — Final review, cache completion, and handoff

Status: `pending`

Depends on: `WI-09`

### Final review

- Inspect `git diff` and `git diff --check`.
- Confirm no secrets, generated contract drift, unrelated formatting churn, or accidental protocol changes.
- Confirm every completed item has a cache file under `docs/agent-cache/history-ui-audit/`.
- Confirm this plan's status table matches cache statuses.
- Confirm commit messages follow the required `+ action: description` format and contain no attribution trailer.
- If UI files changed, run the Impeccable detector once over changed UI targets and cache the result; detector findings are evidence to triage, not a reason to add decorative UI.

### Handoff output

The final agent must report:

- completed work items;
- files changed;
- tests and viewport evidence;
- known unrelated failures;
- remaining work items;
- commit hashes;
- cache paths for every completed item.

## Style impact matrix

| Style group | Selectors/classes | Risk | Work items |
| --- | --- | --- | --- |
| Score trigger/status | `.score-sheet-trigger`, `.score-summary`, `.active-table-client .score-summary` | compact status can become a button that dominates table | WI-02, WI-05, WI-07 |
| Popover shell | `.score-sheet-popover`, `::backdrop` | cascade duplication, mobile inset, max-height, overflow | WI-02, WI-05, WI-07 |
| Shared header | `.score-sheet-header`, `h2`, `.score-sheet-close` | replay overrides can leak into ScoreSheet | WI-01, WI-02, WI-05, WI-06 |
| Score table | `.score-sheet-table-wrap`, `.score-sheet-table`, `caption`, `th`, `td`, `thead th` | sticky header and narrow width/overflow | WI-02, WI-05, WI-07 |
| Replay shell | `.board-replay-modal`, `::backdrop` | native top-layer/modal versus mobile non-modal behavior | WI-01, WI-02, WI-06, WI-07 |
| Replay status | `.replay-message` | loading/error shifts and announcement clarity | WI-01, WI-06, WI-07 |
| Replay geometry | `.replay-surface-wrap`, `> .table-surface` | transform scaling, clipping, resize/orientation | WI-02, WI-04, WI-06, WI-07 |
| Auction scroll | `.auction-table-wrap`, `.auction-table` | nested scroll traps and clipped auction | WI-02, WI-04, WI-07 |
| Replay navigation | `.replay-navigation`, `span`, `button` | navigation hidden or unreachable under scaled surface | WI-01, WI-04, WI-06, WI-07 |
| Shared table geometry | `.table-surface`, `:is(.active-table-client, .replay-surface-wrap)` | active gameplay regression | WI-02, WI-07, WI-09 |
| Cards/hands/trick | `.bridge-hand`, `.hand-card-slot`, `.current-trick`, `.physical-card` | overlap and scale changes across contexts | WI-02, WI-04, WI-07, WI-09 |
| Board result | `.board-result`, active-table result overrides | result state may break stable shell | WI-02, WI-07, WI-09 |
| History master list | `.history-workspace`, `.history-section-heading`, `.history-board-list`, `.history-board-row` | mobile scanability and selected state | WI-01, WI-02, WI-08 |
| History controls | `.history-search-field`, `.history-board-label`, `.history-board-actions`, `.history-replay-button` | focus, error, and disabled-state clarity | WI-01, WI-08 |
| Table viewport | `.active-table-client`, `.play-board-client`, `.gameplay-region` | document overflow and chat/game geometry | WI-02, WI-04, WI-07, WI-09 |

## Explicit non-goals

- Do not redesign the whole app shell or landing page.
- Do not change P6-07 API/query/migration behavior.
- Do not add a global modal manager.
- Do not add tab/filter/expand/collapse to replay unless a separate product decision explicitly requests it.
- Do not add timestamp/player/vulnerability/trick fields to ScoreSheet merely because they are absent; first establish a product requirement and authoritative data source.
- Do not use the audit as a reason to rewrite working gameplay components.

## Definition of done

This plan is complete only when the requested History UI behavior is implemented and evidenced, not when CSS compiles. The final evidence must include focused tests, type/lint/build checks appropriate to the changed files, browser screenshots/geometry checks across the WI-04 matrix, privacy/read-only confirmation, cache files for each completed work item, and a truthful list of residual failures.
