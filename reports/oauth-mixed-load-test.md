# Backend Concurrency Load Test Report

- Generated at: `2026-07-04T13:21:07+08:00`
- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`
- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites, 1 pre-joined Yggdrasil session
- OAuth seed: 1 active confidential client, 2 delegated bearer tokens, 1 client-credentials-style bearer token
- Fixed concurrency: `200`
- Duration per level: `1s`
- Backend database pool used by harness: `20` max connections
- Test database: isolated `elementskin_go_test_*`, dropped by test cleanup
- Redis: real test Redis with isolated `elementskin:test:*` key prefix, cleaned by test cleanup
- Auth rate limiting: disabled for load-test login scenario to measure login throughput instead of 429 policy

## Scenario Coverage

| Area | Scenario | Method | Path |
| --- | --- | --- | --- |
| Public home | `public-settings` | `GET` | `/v2/public/settings` |
| Public home | `public-homepage-media` | `GET` | `/v2/public/homepage-media` |
| Public library | `public-library-search` | `GET` | `/v2/public/skin-library?limit=20&q=Load` |
| Authentication | `site-login` | `POST` | `/v2/auth/login` |
| Yggdrasil | `ygg-metadata` | `GET` | `/` |
| Yggdrasil | `ygg-authenticate` | `POST` | `/authserver/authenticate` |
| Yggdrasil | `ygg-validate` | `POST` | `/authserver/validate` |
| Yggdrasil | `ygg-profile` | `GET` | `/sessionserver/session/minecraft/profile/7fea891c429f497f880af20f6a864557` |
| Yggdrasil | `ygg-lookup-name` | `GET` | `/api/users/profiles/minecraft/LoadProfile002_0` |
| Yggdrasil | `ygg-has-joined` | `GET` | `/sessionserver/session/minecraft/hasJoined?username=LoadProfile002_0&serverId=load_ygg_server` |
| User center | `me` | `GET` | `/v2/users/me` |
| OAuth delegated | `oauth-me` | `GET` | `/v2/users/me` |
| User center | `my-profiles` | `GET` | `/v2/users/me/profiles?limit=20` |
| OAuth delegated | `oauth-my-profiles` | `GET` | `/v2/users/me/profiles?limit=20` |
| User center | `my-textures` | `GET` | `/v2/users/me/textures?limit=20` |
| OAuth delegated | `oauth-my-textures` | `GET` | `/v2/users/me/textures?limit=20` |
| User center | `texture-detail` | `GET` | `/v2/users/me/textures/load_texture_001_000/skin` |
| OAuth delegated | `oauth-texture-detail` | `GET` | `/v2/users/me/textures/load_texture_001_000/skin` |
| Admin console | `admin-users` | `GET` | `/v2/admin/users?limit=20&q=Load` |
| OAuth delegated admin | `oauth-admin-users` | `GET` | `/v2/admin/users?limit=20&q=Load` |
| Admin console | `admin-user-detail` | `GET` | `/v2/admin/users/cef8247ac917451bad64a9d08f530f82` |
| OAuth delegated admin | `oauth-admin-user-detail` | `GET` | `/v2/admin/users/cef8247ac917451bad64a9d08f530f82` |
| Admin console | `admin-user-profiles` | `GET` | `/v2/admin/users/cef8247ac917451bad64a9d08f530f82/profiles?limit=20` |
| Admin console | `admin-profiles` | `GET` | `/v2/admin/profiles?limit=20` |
| Admin console | `admin-textures` | `GET` | `/v2/admin/textures?limit=20` |
| Admin console | `admin-invites` | `GET` | `/v2/admin/invites?limit=20` |
| OAuth delegated admin | `oauth-admin-invites` | `GET` | `/v2/admin/invites?limit=20` |
| OAuth client credentials | `oauth-client-invites` | `GET` | `/v2/admin/invites?limit=20` |
| Admin console | `admin-settings-site` | `GET` | `/v2/admin/settings/site` |
| OAuth delegated admin | `oauth-admin-settings-site` | `GET` | `/v2/admin/settings/site` |

## Fixed-200 One-Second Results

| Area | Scenario | Concurrency | Requests | OK | Fail | Fail % | Successful req/s | Total req/s | Avg | P50 | P95 | P99 | Status | First Error |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| Public home | `public-settings` | 200 | 25087 | 25087 | 0 | 0.00 | 24922.6 | 24922.6 | 8.0ms | 6.1ms | 8.2ms | 10.2ms | `200:25087` | `` |
| Public home | `public-homepage-media` | 200 | 33553 | 33553 | 0 | 0.00 | 33337.8 | 33337.8 | 6.0ms | 5.8ms | 7.7ms | 8.8ms | `200:33553` | `` |
| Public library | `public-library-search` | 200 | 15572 | 15572 | 0 | 0.00 | 15330.1 | 15330.1 | 12.8ms | 12.5ms | 18.6ms | 21.7ms | `200:15572` | `` |
| Authentication | `site-login` | 200 | 364 | 364 | 0 | 0.00 | 243.2 | 243.2 | 725.2ms | 749.7ms | 1.27s | 1.33s | `200:364` | `` |
| Yggdrasil | `ygg-metadata` | 200 | 28164 | 28164 | 0 | 0.00 | 27998.5 | 27998.5 | 7.1ms | 6.7ms | 9.3ms | 14.4ms | `200:28164` | `` |
| Yggdrasil | `ygg-authenticate` | 200 | 316 | 316 | 0 | 0.00 | 285.6 | 285.6 | 686.4ms | 625.0ms | 1.10s | 1.10s | `200:316` | `` |
| Yggdrasil | `ygg-validate` | 200 | 15848 | 15848 | 0 | 0.00 | 15699.8 | 15699.8 | 12.6ms | 12.6ms | 14.2ms | 15.6ms | `204:15848` | `` |
| Yggdrasil | `ygg-profile` | 200 | 60397 | 60397 | 0 | 0.00 | 60225.1 | 60225.1 | 3.2ms | 3.0ms | 5.2ms | 6.7ms | `200:60397` | `` |
| Yggdrasil | `ygg-lookup-name` | 200 | 69130 | 69130 | 0 | 0.00 | 68911.6 | 68911.6 | 2.8ms | 2.6ms | 4.5ms | 5.8ms | `200:69130` | `` |
| Yggdrasil | `ygg-has-joined` | 200 | 1873 | 1873 | 0 | 0.00 | 1622.5 | 1622.5 | 111.3ms | 101.5ms | 260.6ms | 330.8ms | `200:1873` | `` |
| User center | `me` | 200 | 12929 | 12929 | 0 | 0.00 | 12779.5 | 12779.5 | 15.5ms | 15.3ms | 18.7ms | 23.6ms | `200:12929` | `` |
| OAuth delegated | `oauth-me` | 200 | 9451 | 9451 | 0 | 0.00 | 9299.1 | 9299.1 | 21.3ms | 21.0ms | 27.9ms | 36.1ms | `200:9451` | `` |
| User center | `my-profiles` | 200 | 15200 | 15200 | 0 | 0.00 | 15059.7 | 15059.7 | 13.2ms | 13.2ms | 15.0ms | 16.8ms | `200:15200` | `` |
| OAuth delegated | `oauth-my-profiles` | 200 | 12350 | 12350 | 0 | 0.00 | 12230.6 | 12230.6 | 16.2ms | 16.1ms | 20.1ms | 24.2ms | `200:12350` | `` |
| User center | `my-textures` | 200 | 15681 | 15681 | 0 | 0.00 | 15562.5 | 15562.5 | 12.7ms | 12.7ms | 14.8ms | 17.1ms | `200:15681` | `` |
| OAuth delegated | `oauth-my-textures` | 200 | 11675 | 11675 | 0 | 0.00 | 11424.7 | 11424.7 | 17.2ms | 17.1ms | 21.3ms | 24.6ms | `200:11675` | `` |
| User center | `texture-detail` | 200 | 13685 | 13685 | 0 | 0.00 | 13539.0 | 13539.0 | 14.6ms | 14.1ms | 17.9ms | 39.4ms | `200:13685` | `` |
| OAuth delegated | `oauth-texture-detail` | 200 | 11399 | 11399 | 0 | 0.00 | 11258.2 | 11258.2 | 17.6ms | 17.5ms | 22.0ms | 26.0ms | `200:11399` | `` |
| Admin console | `admin-users` | 200 | 3218 | 3218 | 0 | 0.00 | 3057.9 | 3057.9 | 64.1ms | 65.2ms | 86.9ms | 116.0ms | `200:3218` | `` |
| OAuth delegated admin | `oauth-admin-users` | 200 | 2832 | 2832 | 0 | 0.00 | 2730.3 | 2730.3 | 71.8ms | 74.3ms | 98.2ms | 109.9ms | `200:2832` | `` |
| Admin console | `admin-user-detail` | 200 | 12238 | 12238 | 0 | 0.00 | 12048.3 | 12048.3 | 16.4ms | 16.2ms | 20.5ms | 23.7ms | `200:12238` | `` |
| OAuth delegated admin | `oauth-admin-user-detail` | 200 | 9947 | 9947 | 0 | 0.00 | 9782.4 | 9782.4 | 20.2ms | 20.0ms | 27.2ms | 31.4ms | `200:9947` | `` |
| Admin console | `admin-user-profiles` | 200 | 14866 | 14866 | 0 | 0.00 | 14715.3 | 14715.3 | 13.5ms | 13.4ms | 15.4ms | 17.3ms | `200:14866` | `` |
| Admin console | `admin-profiles` | 200 | 13836 | 13836 | 0 | 0.00 | 13654.8 | 13654.8 | 14.5ms | 14.4ms | 17.5ms | 20.2ms | `200:13836` | `` |
| Admin console | `admin-textures` | 200 | 13823 | 13823 | 0 | 0.00 | 13662.7 | 13662.7 | 14.5ms | 14.2ms | 18.1ms | 21.7ms | `200:13823` | `` |
| Admin console | `admin-invites` | 200 | 13013 | 13013 | 0 | 0.00 | 12855.8 | 12855.8 | 15.4ms | 15.0ms | 20.0ms | 26.4ms | `200:13013` | `` |
| OAuth delegated admin | `oauth-admin-invites` | 200 | 10300 | 10300 | 0 | 0.00 | 10136.6 | 10136.6 | 19.4ms | 19.2ms | 25.8ms | 30.4ms | `200:10300` | `` |
| OAuth client credentials | `oauth-client-invites` | 200 | 6834 | 6834 | 0 | 0.00 | 6676.0 | 6676.0 | 29.6ms | 28.6ms | 40.2ms | 44.1ms | `200:6834` | `` |
| Admin console | `admin-settings-site` | 200 | 2400 | 2400 | 0 | 0.00 | 2290.4 | 2290.4 | 86.7ms | 87.0ms | 95.1ms | 99.2ms | `200:2400` | `` |
| OAuth delegated admin | `oauth-admin-settings-site` | 200 | 2400 | 2400 | 0 | 0.00 | 2228.1 | 2228.1 | 89.2ms | 89.5ms | 94.4ms | 97.2ms | `200:2400` | `` |

## Notes

- Every scenario is measured once at the same fixed concurrency, default `200`, for a one-second window.
- `Successful req/s` is the useful per-second throughput under that fixed concurrency.
- This report covers public, site-cookie, OAuth delegated, OAuth client-credentials-style, admin, and common Yggdrasil client endpoints; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.

## Cookie vs OAuth Read Path

| Pair | Cookie req/s | OAuth req/s | OAuth / Cookie | Cookie P95 | OAuth P95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `/v2/users/me` | 12779.5 | 9299.1 | 72.8% | 18.7ms | 27.9ms |
| `/v2/users/me/profiles` | 15059.7 | 12230.6 | 81.2% | 15.0ms | 20.1ms |
| `/v2/users/me/textures` | 15562.5 | 11424.7 | 73.4% | 14.8ms | 21.3ms |
| `/v2/users/me/textures/{hash}/skin` | 13539.0 | 11258.2 | 83.2% | 17.9ms | 22.0ms |
| `/v2/admin/users?q=Load` | 3057.9 | 2730.3 | 89.3% | 86.9ms | 98.2ms |
| `/v2/admin/users/{id}` | 12048.3 | 9782.4 | 81.2% | 20.5ms | 27.2ms |
| `/v2/admin/invites` | 12855.8 | 10136.6 | 78.8% | 20.0ms | 25.8ms |
| `/v2/admin/settings/site` | 2290.4 | 2228.1 | 97.3% | 95.1ms | 94.4ms |

OAuth delegated bearer paths have zero failures at concurrency 200. The added cost is visible but bounded: most user/admin read paths retain roughly 73-89% of cookie throughput, with P95 increases around 4-11ms. The settings endpoint is dominated by its own service work and shows almost no OAuth-specific loss.

`oauth-client-invites` reached 6676.0 successful req/s, 51.9% of the cookie admin invite path, with 40.2ms P95. That path exercises the client principal lookup plus client permission intersection instead of a user delegated grant, and is the current OAuth outlier.
