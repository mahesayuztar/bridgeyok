# ADR 0017 — Participant-only completed board replay

Status: accepted, 2026-09-07

## Decision

Expose `GET /v1/boards/{boardId}/replay` using the existing bearer session. The repository checks current table participation and completed board status in a read-only repeatable-read transaction. Unfinished boards return 409; outsiders and unavailable historical boards return 404. All responses use `Cache-Control: private, no-store`. No archive, deal, credential, or internal error content is logged.

Read permanent `board_records` through the existing Go replay and final-hash validation. For a scored current board before archival, read its matching private snapshot and original deal within the same transaction. This avoids a race with next-board compaction and preserves current scored-board undo behavior. An older board without an archive or matching snapshot is unavailable; this endpoint does not backfill records. This extends ADR 0016's repository-only history access without altering its finality or retention boundaries.

Return the original four hands and final authoritative engine state. Completed tricks and auction reflect accepted undo; claim results retain the actual played tricks without synthesizing awarded tricks. Browser replay only selects a recorded trick and filters its played cards from the original hands. It does not reimplement rules, determine winners, score contracts, or send gameplay commands.

Score History rows open a native modal dialog with one shared TableSurface, canonical BridgeHand/PlayingCard, AuctionTable at trick 0 only, and the existing BoardResult in persistent mode. Board-start seat labels come from the score entry. Starting trick 1 replaces the auction with centered trick cards. The footer contains only navigation arrows and numeric progress. Navigation runs from the complete auction/original deal through recorded tricks to the result; previous navigation restores hands. The result screen reveals the complete original deal again, matching the live completed-board presentation. Escape/close restores focus to the selected history row. Live reconciliation continues independently.

## Verification

API tests cover authorization, unfinished boards, unavailable archives, private errors, and no-store responses. Repository integration compares current snapshots and archived replay for pass-out, undo, thirteen tricks, and accepted claims. Client tests cover rewind and zero-trick endings. Browser checks cover real History row clicks, four hands, auction only at trick 0, all 13 tricks, full-deal result, reopening, focus, and desktop/tablet/mobile geometry (1920×1080, 1024×768, 768×1024, 390×844, 320×700). Dedicated replay E2E, 52 web unit tests, generated contracts, API race tests, vet/lint, and isolated PostgreSQL 17 integration pass.
