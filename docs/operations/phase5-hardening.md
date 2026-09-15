# Phase 5 hardening — 16 September 2026

## Scope and reproducibility

Run `./scripts/harden-phase5.sh` from the repository root. Use its optional `process` argument to rerun only the process drill after lifecycle tests have already passed. It requires Docker, Go, Python 3, OpenSSL and the existing Goose tooling. The runner creates an isolated PostgreSQL 17 container with a preselected loopback port that stays fixed across database restarts and disposable password, applies migrations 00001–00011, builds the current API with the race detector, and builds a rollback candidate from `PHASE5_ROLLBACK_REF` (default: committed HEAD) in a temporary checkout. It never connects to the configured production database. Its exit trap removes the test container and temporary binaries.

The process test is opt-in: ordinary integration suites skip it unless the runner supplies the executable and its owned `bridgeyok-hardening-*` container. It binds an ephemeral loopback HTTP port. The simulated workload uses 4 matches, 8 rooms and 32 distinct registered players, two passed-out boards per match, an API pool of 12 connections, and at most 64 WebSocket connections. This is a bounded local smoke, not a production capacity guarantee or pilot with real testers. DDS performance and full card-play throughput are outside this workload.

## Recovery checks

- Eight concurrent creates sharing the same roster must admit exactly four and reject four with capacity errors.
- An injected SQL failure at the last create write must leave no orphan room; an injected cancellation failure must restore both table snapshots. Retry succeeds after removing the constraint.
- Twenty-two newer cancellations must not hide older unfinished matches from the bounded participant list.
- SIGKILL the API after an accepted call in every room. Restart the process, reconnect all players, and retain the exact durable revision, game and hand state.
- Stop only the owned test database. Readiness and authenticated mutations must return 503, without reclassifying valid credentials as invalid. Restart the database, reconnect and finish every room. The test resets its own inspection pool after the outage; the API process and its pool must recover without being restarted.
- Run the eight rooms concurrently to finalization. Retain exactly two comparisons per match and the expected zero IMP totals.
- Gracefully stop the current API and boot the previously committed compatible binary against the unchanged database. All final match projections must remain identical.
- Check recipient room/session/hand boundaries during recovery and ensure process logs contain no test credentials/session IDs or race reports.

## Fixes found during hardening

`ListParticipantMatchIDs` now orders unfinished matches before recent history. The four-match invitation cap ensures unfinished entries cannot be displaced by the twenty-row history limit.

Shared HTTP authentication now distinguishes invalid/inactive credentials (401) from database/infrastructure errors (retryable 503). Previously, a database outage could be reported as an invalid token and cause unnecessary sign-in recovery.

## Rollback and release limits

Application rollback must retain migrations 00010–00011 and match-aware lifecycle/privacy guards. Never use schema down as a match rollback. A casual-only API must not serve bound rooms. If a compatible build is unavailable, stop traffic and recover the compatible service before reopening rooms.

The runner's rollback candidate establishes compatibility for that exact commit only. It does not authorize arbitrary historic or production rollback builds. Do not deploy multiple API instances: actors, subscriptions and broadcasts remain process-local. Supported single-instance hosting, independent WBF review and a real closed-beta pilot remain release gates. This work performs no production migration, deployment, external invitation or AI implementation.

## Evidence

Local hardening PASS on 16 September 2026:

| Check | Result |
| --- | --- |
| Concurrent invitation capacity, create/cancel SQL rollback, unfinished discovery after 22 cancellations | PASS, 23.56 s |
| Four-match process crash, database outage, reconnect, concurrent room completion, compatible rollback | PASS, 132.43 s |
| ACK timings across 32 sockets, 272 samples, race-instrumented local process | p50 64.3 ms; p95 310.6 ms; max 758.1 ms |
| Rollback candidate | `020046729fcbd98c14e52ba9e0fcdd39f8d8927f`; final projections identical |
| API unit/race suite, Go vet including integration tags, normal API lint | PASS |
| New integration-test lint relative to `0200467` | PASS, zero issues |
| Contract tests | PASS, 4 tests |
| govulncheck v1.7.0 | No vulnerabilities found |
| pnpm production audit | Zero advisories across 97 dependencies including optional dependencies |
| Gitleaks v8.24.3, redacted Git history scan | No leaks found; final scan recorded in local log |

Reproduce the measured process/rollback run with `PHASE5_ROLLBACK_REF=0200467 ./scripts/harden-phase5.sh process`; omit `process` to include lifecycle tests. These timings include the test's frame/privacy checks and are not production latency or load guarantees. Rollback verifies final-state hydration; active gameplay across differing release versions was not exercised.

The process logs were checked for test access tokens/session IDs, the authentication secret and race reports. Recipient snapshots after recovery retained the original seat and immutable source hand, with no opponent hand/dummy/full deal during auction. No UI changed, so the already-passing responsive browser suite was not repeated.

The first process harness attempts exposed invalid subscribe-envelope fields, Docker's remapped ephemeral port, and stale connections in the test's own inspection pool. The runner now uses a stable selected host port, valid protocol envelopes and a refreshed inspection pool after outage. The application pool is not reset by the test. The final run above passes with these fixture corrections.

Local command output is retained under ignored `tmp/phase5-hardening/` so interrupted work can resume. The runner removed its container and temporary binaries after the run. The existing chat and earlier Phase 5 databases were untouched.

## Remaining release work

| Gate | Required evidence | Current state |
| --- | --- | --- |
| Hosting | One persistent API process for actors/WebSockets; verified HTTPS/WSS routing and Origin allowlist; no unrestricted scale-out | Not verified in production |
| Independent bridge review | Experienced reviewer signs the existing WBF rules/scoring matrix with name, date and exceptions | Awaiting reviewer |
| Human pilot | Eight testers complete a match, reconnect during play and confirm final comparison on the chosen host | Not performed |

Start the human pilot with one two-room match on the supported host. Every player must have their own account and explicitly mark ready. Explain the fixed lineup, unavailable active cancellation/substitution, and the room owner's final-board “Selesaikan room” action before play. Record the match ID, deployed commit, client/browser versions, reconnect observations and final comparison. Stop expansion if correctness, missing data or hidden-hand leakage is observed; preserve the match records and use a compatible application recovery build.
