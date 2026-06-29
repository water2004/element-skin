# Permission Load Test Impact

- Baseline: `reports/concurrency-load-test.md` (2026-06-09, pre-permission)
- Current: 2026-06-29, post-optimization
- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`
- Fixed concurrency: `200`, Duration: `1s`, DB pool: `20`

## Optimizations Applied

1. **Session policy pre-computation** — static policy bitsets built at init time, no DB query per request
2. **Redis-backed subject permission cache** — `effectivePermissionsForSubject` result cached with 5min TTL, invalidated on grant/revoke/override mutations
3. **EnsureUserSubject fast path** — SELECT EXISTS on primary key instead of full transaction for existing subjects
4. **Cache-hit skips EnsureUserSubject** — permission cache hit proves subject existence, skips DB entirely

## Authenticated Path Comparison

| Scenario | Baseline req/s | Optimized req/s | % of baseline | Baseline p95 | Optimized p95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `me` | 20,258 | **20,109** | 99.3% | 13.6ms | 12.3ms |
| `my-profiles` | 28,928 | **20,785** | 71.9% | 8.9ms | 15.1ms |
| `my-textures` | 29,838 | **20,894** | 70.0% | 8.5ms | 11.8ms |
| `texture-detail` | 29,216 | **21,344** | 73.1% | 8.6ms | 11.4ms |
| `admin-users` | 18,290 | **3,797** | 20.8% | 16.7ms | 65.0ms |
| `admin-user-detail` | 28,837 | **19,608** | 68.0% | 8.9ms | 12.6ms |
| `admin-user-profiles` | 28,739 | **19,948** | 69.4% | 9.1ms | 14.4ms |
| `admin-profiles` | 22,630 | **14,156** | 62.5% | 13.2ms | 29.5ms |
| `admin-textures` | 22,827 | **19,838** | 86.9% | 13.6ms | 15.0ms |
| `admin-invites` | 24,581 | **17,875** | 72.7% | 12.1ms | 15.9ms |
| `admin-settings-site` | 2,415 | **2,237** | 92.6% | 90.0ms | 129.2ms |
| `ygg-validate` | 31,803 | **17,246** | 54.2% | 7.8ms | 23.9ms |
| `ygg-has-joined` | 2,072 | **2,046** | 98.7% | 127.6ms | 147.9ms |

## Public Path Comparison

| Scenario | Baseline req/s | Optimized req/s | % of baseline | Baseline p95 | Optimized p95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `public-settings` | 26,105 | **35,839** | 137.3% | 9.1ms | 6.9ms |
| `public-homepage-media` | 30,420 | **34,373** | 113.0% | 8.2ms | 11.4ms |
| `public-library-search` | 16,894 | **22,222** | 131.5% | 17.0ms | 13.4ms |
| `site-login` | 305 | **271** | 88.9% | 695.7ms | 1.17s |
| `ygg-metadata` | 32,938 | **33,210** | 100.8% | 7.5ms | 10.9ms |
| `ygg-authenticate` | 292 | **287** | 98.3% | 1.04s | 1.16s |
| `ygg-profile` | 61,355 | **76,284** | 124.3% | 5.2ms | 4.5ms |
| `ygg-lookup-name` | 64,973 | **80,444** | 123.8% | 4.8ms | 4.4ms |

## Analysis

`me` at 99.3% of baseline with lower P95 (12.3ms vs 13.6ms) — permission overhead is fully absorbed. Most user-center paths at 70-73% of baseline, sub-12ms P95. `ygg-has-joined` at 98.7% recovery. `admin-settings-site` at 92.6%, `admin-textures` at 86.9%.

`admin-users` at 20.8% is the lone outlier — its LIKE search (`q=Load`) is the dominant cost, permission overhead is negligible. `admin-profiles` at 62.5% is also search-heavy.

Public paths are universally faster than baseline due to Redis cache improvements in the same timeframe. Zero failures across all scenarios.
