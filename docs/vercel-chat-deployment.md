# Vercel chat deployment readiness

Last updated: 14 September 2026. Branch: main. Continuation of chat JOB-01–08, with explicit authorization to alter Supabase. Status: **not cleared for unrestricted multi-instance production deployment**.

## Changes completed

- API startup and `/health/ready` require the table, account and chat tables plus the cleanup function. The former check only required the schema namespace, allowing readiness to pass before chat migration.
- WebSocket frame configuration now uses at least 32768 bytes. Deployment examples and the Vercel image use that limit. Existing explicit 8192-byte overrides are raised automatically for compatibility, without blocking startup. Values outside the existing configuration range remain invalid.
- The Vercel image explicitly sets port 8080, binds 0.0.0.0, defaults to production and runs as its existing unprivileged user. Project environment variables can override image defaults.
- Vercel web configuration rejects missing or non-HTTPS public API origins and mismatched server/browser API URLs. Seven configuration tests cover rejection and valid fallback.
- sqlc models were regenerated against migrations 00008–00009.

## Supabase changes executed

The configured Supabase session-pooler target already had migrations 00001–00009; no migration was reapplied. `chat_messages` RLS was enabled, `player_invites` was absent, and anon/authenticated roles could not execute cleanup.

Installed pg_cron 1.6.4 and named job `bridgeyok-chat-retention`: `17 3 * * *`, active, database postgres, command `SELECT bridgeyok.cleanup_chat_messages()`. This is 03:17 UTC / 10:17 WIB under the scheduler default timezone. The cleanup function executed successfully; no expired messages existed at inspection. A temporary five-second job invoked cleanup and unscheduled itself. `cron.job_run_details` recorded success at 2026-09-14 11:02:03 UTC; only the daily job remained. No chat content, account records or credentials were printed or exported.

`db/operations/chat-retention.sql` is repeatable by job name, transactional, and bounded by lock/statement timeouts. Production cleanup installation is complete.

## Verification

- Live production: API live/ready both 200; chat history without credentials 401; WS without a ticket 401; foreign Origin 403; allowed web Origin gets the expected CORS header.
- API config/httpapi/realtime race suites pass; Go vet and API lint pass (zero issues).
- Local PostgreSQL readiness integration passes, including renaming chat storage inside a rolled-back transaction to prove missing migration fails readiness. Initial run found the local container stopped; after starting it, both tests passed. No integration test altered production tables.
- `docker build -f Dockerfile.vercel -t bridgeyok-api:vercel-check .` passes. The image starts against isolated local PostgreSQL, returns ready 200, runs as UID 999 and stops cleanly.
- All three DDS golden fixtures pass inside that image with 256 MB memory, one CPU and zero-swap emulation.
- Vercel-mode web production build with production API URLs passes; TypeScript and ESLint pass. Seven URL configuration tests pass.
- Earlier chat delivery/retention/responsive browser evidence remains in `docs/chat-implementation-validation.md`; this follow-up did not change gameplay or chat presentation.

## Remaining deployment blocker

Vercel's current documentation supports both container images and WebSockets. However, connections are pinned only individually; different users and reconnects may reach different function instances. Containers scale automatically and old/new deployments may overlap. Sources: [WebSockets](https://vercel.com/docs/functions/websockets), [Container Images](https://vercel.com/docs/functions/container-images).

Code inspection shows `realtime.Server.connections`, chat/social broadcast recipients, and table actor subscriptions are process-local. Therefore, we infer that users on different instances can miss live chat/social/gameplay broadcasts even though persistence and HTTP health succeed. This is an architectural risk identified from code and platform behavior, not a claim that a production multi-instance incident was reproduced. A region setting such as sin1 does not prove one shared process. Per-user sticky routing alone does not place every table participant or friend on the same process.

There are no Vercel credentials, linked project metadata or Vercel connector in this workspace. Consequently deployment settings, active revision, maximum duration and actual instance routing remain unverified. No deploy or Git push was performed.

Before promotion, establish one authoritative long-lived API process reachable by all clients, or implement and test cross-instance table ownership and pub/sub using the existing PostgreSQL boundary. The existing single-VM API deployment is one documented hosting option; moving endpoints or adding infrastructure was not performed. Do not merely limit concurrency per function or add chat-only pub/sub and claim gameplay is fixed.

For a Vercel-native multi-instance implementation, verify authenticated clients on distinct instances, private/table/social delivery, controller ownership, concurrent game commands, rolling deploy/reconnect and bounded database connection usage. Notifications must remain non-persistent. A dedicated PostgreSQL LISTEN connection would require session/direct pooling and explicit lifecycle/recovery design; no Redis or second socket service is authorized by the chat scope.

## Required project configuration

API project root: repository root, `Dockerfile.vercel`; web project root: `apps/web`, with workspace dependencies available. Set API `APP_ENV=production`, `API_HOST=0.0.0.0`, `PORT=8080`, `REALTIME_READ_LIMIT_BYTES=32768`, `ALLOWED_ORIGINS=https://bridgeyok-web.vercel.app`, stable strong `AUTH_SECRET`, and a TLS Supabase session-pooler `DATABASE_URL`. Budget the database pool across every possible process; the application default is five connections per process.

Web: `NEXT_PUBLIC_API_BASE_URL=https://bridgeyok-api.vercel.app`; omit `API_BASE_URL` or set it to the same URL. Rebuild after changing NEXT_PUBLIC variables. Preview deployments must use an isolated database/secret/API and an explicit matching Origin allowlist.

Next action: obtain authenticated Vercel project access, verify deployment topology, resolve the process-local realtime blocker, then run the production promotion checks. Supabase permission is already granted and must not be requested again.


## Login incident follow-up — 14 September 2026

Subsequent live checks returned Vercel FUNCTION_INVOCATION_FAILED (500) from API `/health/live`, `/health/ready`, and POST `/v1/account/login`. Frontend POST `/api/account/login` propagates the upstream failure. User-provided frontend logs also show account-service failures; API runtime logs subsequently confirmed the startup cause: deployment dpl_GgrQ978uYQaosWNbLYsi8wEdZypJ rejected REALTIME_READ_LIMIT_BYTES below 32768. The earlier successful health checks above predate this incident.

The frontend route is an intentional same-origin account proxy for cookie handling. The API environment screenshot includes a trailing slash. Replaced string concatenation in both the proxy and server account lookup with URL construction, avoiding `//v1/account`. A local Next.js HTTP smoke test with an upstream stub verifies POST method, request body, query and upstream 401 are preserved with a trailing-slash base URL.

The strict 32 KB minimum added in the deployment follow-up was also revised: a valid legacy 8 KB setting is now upgraded automatically, rather than aborting startup. The supplied API runtime logs confirm the strict check caused the production incident. Fix 3d20bb1 restores compatibility; alternatively, set REALTIME_READ_LIMIT_BYTES=32768 in the API project and redeploy the current revision. Font visibility warnings are unrelated to the API invocation failure. No production deploy was performed by the assistant.
