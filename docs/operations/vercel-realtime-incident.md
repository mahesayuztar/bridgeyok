# Vercel realtime diagnosis — 6 September 2026

Production web: https://bridgeyok-web.vercel.app
Production API: https://bridgeyok-api.vercel.app

The deployed web bundle targets the production API. The API accepts the production web Origin. A two-session browser probe received `table.locked` on both WebSocket connections without refreshing. A real UI seat change also updated without refreshing. Accelerating the existing 285-second rotation timer produced close code 4000 and a successful replacement connection.

Starting a board with one ready human and three bots returned a WebSocket `command.rejected` frame with `INTERNAL_ERROR`. A read-only database transaction found the production probe table, confirmed migrations 00001–00004 were applied, and confirmed `bridgeyok.board_deals` was absent. Migration 00005 is required by `CommandStartGame` and `CommandRequestNextBoard` in `apps/api/internal/database/command_repository.go`. Both operations persist immutable board provenance in the same transaction as the new board, so the missing relation prevents the transition while previously committed scores remain intact.

Apply `db/migrations/00005_board_deals.sql` with the existing Goose migration workflow after production authorization. The forward migration adds the private provenance relation; it does not rewrite existing boards or scores. Verify production start-board and next-board behavior after applying it. Do not run migration down against production.

The reported intermittent failure to receive updates from initial entry has not been reproduced conclusively. The client also ignores close code 1000 without updating connection status, and the backend broker/actor registry are process-local. These are separate investigation paths; neither is proven to explain this incident. A single Vercel region does not establish a single backend process. See the single-instance boundary in PLAN.md and Vercel's [cross-instance WebSocket guidance](https://vercel.com/kb/guide/real-time-chat-websockets).
