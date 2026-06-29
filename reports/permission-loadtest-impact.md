# Permission Load Test Impact

- Baseline report: `reports/concurrency-load-test.md`, generated at `2026-06-09T03:51:56+08:00`
- Current report: `reports/concurrency-load-test-permission-current.md`, generated at `2026-06-29T18:03:10+08:00`
- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`
- Fixed concurrency: `200`
- Duration per scenario: `1s`
- Database pool: `20` max connections

## Summary

Public unauthenticated paths were not harmed by the fine-grained permission model and several are faster than the older baseline. Authenticated user, admin, and permission-gated Yggdrasil session paths regressed sharply. The current implementation appears to spend substantial time rebuilding actor/effective permissions through database-backed checks on each gated request.

Two transient failures were observed in the current run:

- `my-profiles`: `4` failures, `0.15%`, status `200:2734,500:4`
- `ygg-has-joined`: `1` failure, `0.07%`, status `200:1485,500:1`

## Authenticated Path Comparison

| Scenario | Baseline req/s | Current req/s | Current / baseline | Baseline p95 | Current p95 | Current failures |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `me` | 20258.1 | 2431.8 | 12.00% | 13.6ms | 108.5ms | 0.00% |
| `my-profiles` | 28928.8 | 2621.4 | 9.06% | 8.9ms | 90.1ms | 0.15% |
| `my-textures` | 29838.0 | 2598.7 | 8.71% | 8.5ms | 96.2ms | 0.00% |
| `texture-detail` | 29216.8 | 2308.8 | 7.90% | 8.6ms | 124.0ms | 0.00% |
| `admin-users` | 18290.2 | 369.2 | 2.02% | 16.7ms | 729.5ms | 0.00% |
| `admin-user-detail` | 28837.8 | 1707.4 | 5.92% | 8.9ms | 153.2ms | 0.00% |
| `admin-user-profiles` | 28739.6 | 2755.1 | 9.59% | 9.1ms | 95.2ms | 0.00% |
| `admin-profiles` | 22630.1 | 2712.9 | 11.99% | 13.2ms | 84.4ms | 0.00% |
| `admin-textures` | 22827.7 | 2563.2 | 11.23% | 13.6ms | 95.7ms | 0.00% |
| `admin-invites` | 24581.6 | 2979.9 | 12.12% | 12.1ms | 92.2ms | 0.00% |
| `admin-settings-site` | 2415.1 | 1431.1 | 59.26% | 90.0ms | 156.6ms | 0.00% |
| `ygg-validate` | 31803.1 | 2542.1 | 7.99% | 7.8ms | 100.7ms | 0.00% |
| `ygg-has-joined` | 2072.2 | 1306.5 | 63.05% | 127.6ms | 182.1ms | 0.07% |

## Public Path Comparison

| Scenario | Baseline req/s | Current req/s | Current / baseline | Baseline p95 | Current p95 | Current failures |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `public-settings` | 26105.8 | 36404.6 | 139.45% | 9.1ms | 7.1ms | 0.00% |
| `public-homepage-media` | 30420.8 | 40659.7 | 133.66% | 8.2ms | 6.9ms | 0.00% |
| `public-library-search` | 16894.7 | 18078.8 | 107.01% | 17.0ms | 16.9ms | 0.00% |
| `site-login` | 305.6 | 264.1 | 86.42% | 695.7ms | 1.08s | 0.00% |
| `ygg-metadata` | 32938.5 | 42358.1 | 128.60% | 7.5ms | 6.5ms | 0.00% |
| `ygg-authenticate` | 292.1 | 222.2 | 76.07% | 1.04s | 1.14s | 0.00% |
| `ygg-profile` | 61355.0 | 73167.8 | 119.25% | 5.2ms | 4.9ms | 0.00% |
| `ygg-lookup-name` | 64973.6 | 77600.2 | 119.43% | 4.8ms | 4.5ms | 0.00% |

## Follow-Up Direction

The next performance pass should focus on caching an actor's effective permission set and invalidating it when roles or overrides change. The cache should be keyed by user id and permission version, so request handlers can perform one lightweight lookup instead of rebuilding the full permission graph for every protected route.
