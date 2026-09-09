# Phase 4 deal sources and DDS

Apply forward migration 00005 before deploying the new API. It adds private immutable per-board source records; existing games/results remain unchanged. Legacy boards without source records return `BOARD_NOT_FOUND` for analysis. Do not manufacture provenance.

Run `make build-dds`, `make migrate-up`, then `make run-api` from the repository root. The API's default executable path is `./bin/bridgeyok-dds`, relative to its working directory. When launching from another directory, set `DDS_EXECUTABLE` to an absolute path. Defaults: `DDS_TIMEOUT=10s`, `DDS_CONCURRENCY=1`; allowed bounds are a positive timeout ≤60s and 1–4 workers. Keep `WRITE_TIMEOUT` greater than `DDS_TIMEOUT`. Each native worker requests 64 MiB DDS memory, plus process overhead. Capacity is bounded without a waiting queue. The deployment image builds and bundles the pinned executable, Apache license, `libstdc++6`, and `procps` (DDS calls `free` while sizing memory). API image compilation is limited to two Go workers. The local release drill runs `python3 scripts/smoke-dds-image.py IMAGE` to verify the native solver inside the built image before tagging/deploying it.

`GET /v1/boards/{boardId}/analysis` requires a bearer token and takes no deal body. Only current members can analyze a completed board with recorded provenance. Responses include metadata, safe provenance, 20 DD trick counts, maximum makeable levels, and dealer-aware par. They exclude the original pack and raw solver diagnostics. Errors retain `private, no-store` caching policy.

Use `board_analysis_processed` with `request_id` to distinguish `BOARD_NOT_FOUND`, `BOARD_NOT_COMPLETED`, `ANALYSIS_BUSY`, `ANALYSIS_TIMEOUT`, and `ANALYSIS_UNAVAILABLE`. Aggregate `latency_ms` into latency distributions in the JSON-log backend. Source failure is separately recorded as `DEAL_SOURCE_UNAVAILABLE` on the command log. Never enable payload logging to diagnose either failure.

For unavailable analysis, check executable path/permission and the installed C++ runtime, then run `make test-analysis`. For busy responses, retry after two seconds; raise concurrency only after checking memory. Investigate repeated timeouts using nonprivate fixtures. Solver failure cannot change the committed result or block gameplay.

For the Vercel container, build `docker build -f Dockerfile.vercel -t bridgeyok-vercel .` and run `python3 scripts/smoke-dds-image.py bridgeyok-vercel` before deploying. The final runtime stage must include `procps`, even when the DDS binary exists and `ldd` resolves all libraries. Without its `free` command, DDS can exit with `sh: 1: free: not found` and `Memory::GetPtr: 0 vs. 0`; the API maps the process failure to `503 ANALYSIS_UNAVAILABLE`. The `DDS initialized` log only confirms adapter configuration, not native solver readiness.

Verification:

The pinned DDS Linux memory probe originally selected the third line of `free -k`, which is `Swap:` in the Debian runtime. With zero swap it allocates no workers and exits 1 with `Memory::GetPtr: 0 vs. 0` on stdout and empty stderr, even when RAM is available. `scripts/build-dds.sh` patches the probe to select `Mem:` explicitly and read its available-memory column. Keep this patch when rebuilding the pinned solver; increasing `DDS_TIMEOUT` does not address this initialization failure. Verify with `python3 scripts/smoke-dds-image.py IMAGE --zero-swap` as well as the normal image smoke test.

```sh
make test-deal-sources
make test-analysis
TEST_DATABASE_URL="$DATABASE_URL" go test -race -tags=integration ./apps/api/internal/database -run TestBoardSourcesPersistAtomicallyAndAuthorizeAnalysis -count=1
make test-api
```

The DDS gate uses three upstream golden fixtures, including normal and sacrifice contracts. Build downloads are pinned by revision and SHA-256; changes require review of golden differences and ADR-015. The standalone build omits upstream DLL entry points to avoid constructor/finalizer ordering conflicts.

Rollback application binaries while retaining migration 00005; the old API ignores the new table. Migration down is only for isolated tests and removes provenance. Independent experienced-player approval of the WBF boundary is still a Phase 4 exit requirement. Phase 5 must additionally withhold analysis until both match rooms have finished a shared board.
