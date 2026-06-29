# Backend Concurrency Load Test Report

- Generated at: `2026-06-29T23:45:00+08:00` (post-optimization)
- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`
- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites, 1 pre-joined Yggdrasil session
- Fixed concurrency: `200`
- Duration per level: `1s`
- Backend database pool used by harness: `20` max connections
- Test database: isolated `elementskin_go_test_*`, dropped by test cleanup
- Redis: real test Redis with isolated `elementskin:test:*` key prefix, cleaned by test cleanup
- Auth rate limiting: disabled for load-test login scenario to measure login throughput instead of 429 policy

## Scenario Coverage

| Area | Scenario | Method | Path |
| --- | --- | --- | --- |
| Public home | `public-settings` | `GET` | `/public/settings` |
| Public home | `public-homepage-media` | `GET` | `/public/homepage-media` |
| Public library | `public-library-search` | `GET` | `/public/skin-library?limit=20&q=Load` |
| Authentication | `site-login` | `POST` | `/site-login` |
| Yggdrasil | `ygg-metadata` | `GET` | `/` |
| Yggdrasil | `ygg-authenticate` | `POST` | `/authserver/authenticate` |
| Yggdrasil | `ygg-validate` | `POST` | `/authserver/validate` |
| Yggdrasil | `ygg-profile` | `GET` | `/sessionserver/session/minecraft/profile/282392eb7cea4340909d8c76a14c00d5` |
| Yggdrasil | `ygg-lookup-name` | `GET` | `/api/users/profiles/minecraft/LoadProfile002_0` |
| Yggdrasil | `ygg-has-joined` | `GET` | `/sessionserver/session/minecraft/hasJoined?username=LoadProfile002_0&serverId=load_ygg_server` |
| User center | `me` | `GET` | `/me` |
| User center | `my-profiles` | `GET` | `/me/profiles?limit=20` |
| User center | `my-textures` | `GET` | `/me/textures?limit=20` |
| User center | `texture-detail` | `GET` | `/me/textures/load_texture_001_000/skin` |
| Admin console | `admin-users` | `GET` | `/admin/users?limit=20&q=Load` |
| Admin console | `admin-user-detail` | `GET` | `/admin/users/f72f7066154347a88a9b185a64de0d6d` |
| Admin console | `admin-user-profiles` | `GET` | `/admin/users/f72f7066154347a88a9b185a64de0d6d/profiles?limit=20` |
| Admin console | `admin-profiles` | `GET` | `/admin/profiles?limit=20` |
| Admin console | `admin-textures` | `GET` | `/admin/textures?limit=20` |
| Admin console | `admin-invites` | `GET` | `/admin/invites?limit=20` |
| Admin console | `admin-settings-site` | `GET` | `/admin/settings/site` |

## Fixed-200 One-Second Results (Post-Optimization)

| Area | Scenario | Concurrency | OK | Fail | Successful req/s | Avg | P50 | P95 | P99 |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Public home | `public-settings` | 200 | 36031 | 0 | **35,839** | 5.5ms | 4.4ms | 6.9ms | 8.8ms |
| Public home | `public-homepage-media` | 200 | 34574 | 0 | **34,373** | 5.8ms | 4.9ms | 11.4ms | 15.1ms |
| Public library | `public-library-search` | 200 | 22344 | 0 | **22,222** | 8.8ms | 8.7ms | 13.4ms | 16.7ms |
| Authentication | `site-login` | 200 | 401 | 0 | **271** | 621.2ms | 557.8ms | 1.17s | 1.35s |
| Yggdrasil | `ygg-metadata` | 200 | 33363 | 0 | **33,210** | 5.9ms | 5.4ms | 10.9ms | 14.1ms |
| Yggdrasil | `ygg-authenticate` | 200 | 350 | 0 | **287** | 651.3ms | 642.2ms | 1.16s | 1.17s |
| Yggdrasil | `ygg-validate` | 200 | 17446 | 0 | **17,246** | 11.5ms | 9.0ms | 23.9ms | 27.7ms |
| Yggdrasil | `ygg-profile` | 200 | 76549 | 0 | **76,284** | 2.5ms | 2.3ms | 4.5ms | 5.8ms |
| Yggdrasil | `ygg-lookup-name` | 200 | 80563 | 0 | **80,444** | 2.4ms | 2.2ms | 4.4ms | 5.6ms |
| Yggdrasil | `ygg-has-joined` | 200 | 2244 | 0 | **2,046** | 93.9ms | 89.5ms | 147.9ms | 189.9ms |
| User center | `me` | 200 | 20253 | 0 | **20,109** | 9.9ms | 9.7ms | 12.3ms | 14.5ms |
| User center | `my-profiles` | 200 | 20921 | 0 | **20,785** | 9.5ms | 9.1ms | 15.1ms | 18.2ms |
| User center | `my-textures` | 200 | 21049 | 0 | **20,894** | 9.5ms | 9.4ms | 11.8ms | 13.7ms |
| User center | `texture-detail` | 200 | 21463 | 0 | **21,344** | 9.3ms | 9.3ms | 11.4ms | 13.8ms |
| Admin console | `admin-users` | 200 | 3901 | 0 | **3,797** | 52.0ms | 53.1ms | 65.0ms | 68.4ms |
| Admin console | `admin-user-detail` | 200 | 19796 | 0 | **19,608** | 10.1ms | 10.1ms | 12.6ms | 14.7ms |
| Admin console | `admin-user-profiles` | 200 | 20064 | 0 | **19,948** | 9.9ms | 9.7ms | 14.4ms | 17.1ms |
| Admin console | `admin-profiles` | 200 | 14262 | 0 | **14,156** | 14.0ms | 11.3ms | 29.5ms | 39.3ms |
| Admin console | `admin-textures` | 200 | 19980 | 0 | **19,838** | 9.9ms | 9.7ms | 15.0ms | 18.1ms |
| Admin console | `admin-invites` | 200 | 17995 | 0 | **17,875** | 11.1ms | 10.8ms | 15.9ms | 21.1ms |
| Admin console | `admin-settings-site` | 200 | 2319 | 0 | **2,237** | 88.0ms | 86.7ms | 129.2ms | 133.8ms |

## Notes

- Every scenario is measured once at the same fixed concurrency, default `200`, for a one-second window.
- `Successful req/s` is the useful per-second throughput under that fixed concurrency.
- This report covers public, site, admin, and common Yggdrasil client endpoints; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.
