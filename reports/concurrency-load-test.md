# Backend Concurrency Load Test Report

- Generated at: `2026-07-18T02:58:39+08:00`
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
| Yggdrasil | `ygg-profile` | `GET` | `/sessionserver/session/minecraft/profile/c140127903fb4c06a9ae3e8ad8801fec` |
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
| Admin console | `admin-user-detail` | `GET` | `/v2/admin/users/0a8e0f3544794d4a8c6982fbaa1c6f38` |
| OAuth delegated admin | `oauth-admin-user-detail` | `GET` | `/v2/admin/users/0a8e0f3544794d4a8c6982fbaa1c6f38` |
| Admin console | `admin-user-profiles` | `GET` | `/v2/admin/users/0a8e0f3544794d4a8c6982fbaa1c6f38/profiles?limit=20` |
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
| Public home | `public-settings` | 200 | 26879 | 26879 | 0 | 0.00 | 26733.6 | 26733.6 | 7.4ms | 6.0ms | 8.4ms | 42.7ms | `200:26879` | `` |
| Public home | `public-homepage-media` | 200 | 32811 | 32811 | 0 | 0.00 | 32634.7 | 32634.7 | 6.1ms | 5.9ms | 7.8ms | 9.2ms | `200:32811` | `` |
| Public library | `public-library-search` | 200 | 18349 | 18349 | 0 | 0.00 | 18196.2 | 18196.2 | 10.7ms | 10.5ms | 16.1ms | 18.9ms | `200:18349` | `` |
| Authentication | `site-login` | 200 | 387 | 387 | 0 | 0.00 | 311.7 | 311.7 | 597.4ms | 587.7ms | 890.7ms | 1.14s | `200:387` | `` |
| Yggdrasil | `ygg-metadata` | 200 | 26309 | 26309 | 0 | 0.00 | 26109.0 | 26109.0 | 7.6ms | 7.4ms | 10.2ms | 12.5ms | `200:26309` | `` |
| Yggdrasil | `ygg-authenticate` | 200 | 325 | 325 | 0 | 0.00 | 289.6 | 289.6 | 670.8ms | 614.6ms | 1.11s | 1.11s | `200:325` | `` |
| Yggdrasil | `ygg-validate` | 200 | 16343 | 16343 | 0 | 0.00 | 16188.3 | 16188.3 | 12.3ms | 12.2ms | 13.9ms | 15.0ms | `204:16343` | `` |
| Yggdrasil | `ygg-profile` | 200 | 70408 | 70408 | 0 | 0.00 | 70172.7 | 70172.7 | 2.8ms | 2.5ms | 4.6ms | 6.0ms | `200:70408` | `` |
| Yggdrasil | `ygg-lookup-name` | 200 | 75411 | 75411 | 0 | 0.00 | 75233.8 | 75233.8 | 2.6ms | 2.5ms | 4.2ms | 5.5ms | `200:75411` | `` |
| Yggdrasil | `ygg-has-joined` | 200 | 2305 | 2305 | 0 | 0.00 | 1976.9 | 1976.9 | 90.7ms | 85.8ms | 158.5ms | 220.4ms | `200:2305` | `` |
| User center | `me` | 200 | 13048 | 13048 | 0 | 0.00 | 12896.7 | 12896.7 | 15.4ms | 15.2ms | 18.7ms | 21.9ms | `200:13048` | `` |
| OAuth delegated | `oauth-me` | 200 | 9706 | 9706 | 0 | 0.00 | 9588.1 | 9588.1 | 20.7ms | 20.7ms | 28.1ms | 32.0ms | `200:9706` | `` |
| User center | `my-profiles` | 200 | 17240 | 17240 | 0 | 0.00 | 17094.2 | 17094.2 | 11.6ms | 11.6ms | 13.4ms | 15.0ms | `200:17240` | `` |
| OAuth delegated | `oauth-my-profiles` | 200 | 13385 | 13385 | 0 | 0.00 | 13213.5 | 13213.5 | 15.0ms | 14.8ms | 18.9ms | 22.2ms | `200:13385` | `` |
| User center | `my-textures` | 200 | 17214 | 17214 | 0 | 0.00 | 17070.5 | 17070.5 | 11.7ms | 11.6ms | 13.6ms | 16.0ms | `200:17214` | `` |
| OAuth delegated | `oauth-my-textures` | 200 | 11961 | 11961 | 0 | 0.00 | 11809.0 | 11809.0 | 16.8ms | 16.2ms | 23.5ms | 30.9ms | `200:11961` | `` |
| User center | `texture-detail` | 200 | 16783 | 16783 | 0 | 0.00 | 16641.1 | 16641.1 | 11.9ms | 11.9ms | 13.8ms | 15.6ms | `200:16783` | `` |
| OAuth delegated | `oauth-texture-detail` | 200 | 13284 | 13284 | 0 | 0.00 | 13124.8 | 13124.8 | 15.1ms | 14.8ms | 19.1ms | 22.0ms | `200:13284` | `` |
| Admin console | `admin-users` | 200 | 1991 | 1991 | 0 | 0.00 | 1879.5 | 1879.5 | 104.5ms | 103.7ms | 124.8ms | 134.0ms | `200:1991` | `` |
| OAuth delegated admin | `oauth-admin-users` | 200 | 1871 | 1871 | 0 | 0.00 | 1712.3 | 1712.3 | 115.3ms | 116.2ms | 136.3ms | 142.7ms | `200:1871` | `` |
| Admin console | `admin-user-detail` | 200 | 12292 | 12292 | 0 | 0.00 | 12154.6 | 12154.6 | 16.3ms | 16.2ms | 19.4ms | 21.7ms | `200:12292` | `` |
| OAuth delegated admin | `oauth-admin-user-detail` | 200 | 9529 | 9529 | 0 | 0.00 | 9374.1 | 9374.1 | 21.1ms | 20.9ms | 29.7ms | 34.3ms | `200:9529` | `` |
| Admin console | `admin-user-profiles` | 200 | 16529 | 16529 | 0 | 0.00 | 16390.8 | 16390.8 | 12.1ms | 12.1ms | 13.8ms | 15.0ms | `200:16529` | `` |
| Admin console | `admin-profiles` | 200 | 14439 | 14439 | 0 | 0.00 | 14260.2 | 14260.2 | 13.9ms | 13.8ms | 17.0ms | 19.9ms | `200:14439` | `` |
| Admin console | `admin-textures` | 200 | 15122 | 15122 | 0 | 0.00 | 14997.6 | 14997.6 | 13.2ms | 12.9ms | 17.0ms | 26.5ms | `200:15122` | `` |
| Admin console | `admin-invites` | 200 | 14943 | 14943 | 0 | 0.00 | 14821.4 | 14821.4 | 13.4ms | 13.3ms | 16.0ms | 18.5ms | `200:14943` | `` |
| OAuth delegated admin | `oauth-admin-invites` | 200 | 11554 | 11554 | 0 | 0.00 | 11404.4 | 11404.4 | 17.3ms | 17.1ms | 22.5ms | 25.8ms | `200:11554` | `` |
| OAuth client credentials | `oauth-client-invites` | 200 | 6966 | 6966 | 0 | 0.00 | 6709.2 | 6709.2 | 28.9ms | 27.2ms | 42.1ms | 49.1ms | `200:6966` | `` |
| Admin console | `admin-settings-site` | 200 | 2792 | 2792 | 0 | 0.00 | 2607.6 | 2607.6 | 76.2ms | 75.7ms | 80.8ms | 85.8ms | `200:2792` | `` |
| OAuth delegated admin | `oauth-admin-settings-site` | 200 | 2600 | 2600 | 0 | 0.00 | 2525.0 | 2525.0 | 78.8ms | 79.6ms | 83.0ms | 89.7ms | `200:2600` | `` |

## Notes

- Every scenario is measured once at the same fixed concurrency, default `200`, for a one-second window.
- `Successful req/s` is the useful per-second throughput under that fixed concurrency.
- This report covers public, site-cookie, OAuth delegated, OAuth client-credentials-style, admin, and common Yggdrasil client endpoints; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.
