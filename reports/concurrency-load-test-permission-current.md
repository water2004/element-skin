# Backend Concurrency Load Test Report

- Generated at: `2026-06-29T18:03:10+08:00`
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

## Fixed-200 One-Second Results

| Area | Scenario | Concurrency | Requests | OK | Fail | Fail % | Successful req/s | Total req/s | Avg | P50 | P95 | P99 | Status | First Error |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| Public home | `public-settings` | 200 | 36534 | 36534 | 0 | 0.00 | 36404.6 | 36404.6 | 5.4ms | 4.3ms | 7.1ms | 36.7ms | `200:36534` | `` |
| Public home | `public-homepage-media` | 200 | 40916 | 40916 | 0 | 0.00 | 40659.7 | 40659.7 | 4.9ms | 4.5ms | 6.9ms | 12.6ms | `200:40916` | `` |
| Public library | `public-library-search` | 200 | 18188 | 18188 | 0 | 0.00 | 18078.8 | 18078.8 | 10.9ms | 10.6ms | 16.9ms | 22.8ms | `200:18188` | `` |
| Authentication | `site-login` | 200 | 374 | 374 | 0 | 0.00 | 264.1 | 264.1 | 739.4ms | 784.2ms | 1.08s | 1.37s | `200:374` | `` |
| Yggdrasil | `ygg-metadata` | 200 | 42555 | 42555 | 0 | 0.00 | 42358.1 | 42358.1 | 4.7ms | 4.4ms | 6.5ms | 8.2ms | `200:42555` | `` |
| Yggdrasil | `ygg-authenticate` | 200 | 275 | 275 | 0 | 0.00 | 222.2 | 222.2 | 808.1ms | 990.1ms | 1.14s | 1.15s | `200:275` | `` |
| Yggdrasil | `ygg-validate` | 200 | 2702 | 2702 | 0 | 0.00 | 2542.1 | 2542.1 | 77.9ms | 75.3ms | 100.7ms | 148.1ms | `204:2702` | `` |
| Yggdrasil | `ygg-profile` | 200 | 73335 | 73335 | 0 | 0.00 | 73167.8 | 73167.8 | 2.6ms | 2.4ms | 4.9ms | 6.6ms | `200:73335` | `` |
| Yggdrasil | `ygg-lookup-name` | 200 | 77759 | 77759 | 0 | 0.00 | 77600.2 | 77600.2 | 2.5ms | 2.2ms | 4.5ms | 6.2ms | `200:77759` | `` |
| Yggdrasil | `ygg-has-joined` | 200 | 1486 | 1485 | 1 | 0.07 | 1306.5 | 1307.4 | 144.6ms | 142.5ms | 182.1ms | 201.2ms | `200:1485,500:1` | `{"detail":"Internal server error"}` |
| User center | `me` | 200 | 2482 | 2482 | 0 | 0.00 | 2431.8 | 2431.8 | 81.9ms | 80.3ms | 108.5ms | 150.7ms | `200:2482` | `` |
| User center | `my-profiles` | 200 | 2738 | 2734 | 4 | 0.15 | 2621.4 | 2625.3 | 75.7ms | 76.0ms | 90.1ms | 143.6ms | `200:2734,500:4` | `{"detail":"Internal server error"}` |
| User center | `my-textures` | 200 | 2720 | 2720 | 0 | 0.00 | 2598.7 | 2598.7 | 76.6ms | 74.6ms | 96.2ms | 136.2ms | `200:2720` | `` |
| User center | `texture-detail` | 200 | 2353 | 2353 | 0 | 0.00 | 2308.8 | 2308.8 | 85.1ms | 83.3ms | 124.0ms | 139.3ms | `200:2353` | `` |
| Admin console | `admin-users` | 200 | 400 | 400 | 0 | 0.00 | 369.2 | 369.2 | 525.0ms | 393.9ms | 729.5ms | 733.3ms | `200:400` | `` |
| Admin console | `admin-user-detail` | 200 | 1835 | 1835 | 0 | 0.00 | 1707.4 | 1707.4 | 114.2ms | 109.1ms | 153.2ms | 186.6ms | `200:1835` | `` |
| Admin console | `admin-user-profiles` | 200 | 2929 | 2929 | 0 | 0.00 | 2755.1 | 2755.1 | 72.0ms | 68.1ms | 95.2ms | 123.3ms | `200:2929` | `` |
| Admin console | `admin-profiles` | 200 | 2752 | 2752 | 0 | 0.00 | 2712.9 | 2712.9 | 72.9ms | 71.7ms | 84.4ms | 134.2ms | `200:2752` | `` |
| Admin console | `admin-textures` | 200 | 2686 | 2686 | 0 | 0.00 | 2563.2 | 2563.2 | 77.3ms | 75.1ms | 95.7ms | 138.8ms | `200:2686` | `` |
| Admin console | `admin-invites` | 200 | 3046 | 3046 | 0 | 0.00 | 2979.9 | 2979.9 | 66.7ms | 64.2ms | 92.2ms | 119.1ms | `200:3046` | `` |
| Admin console | `admin-settings-site` | 200 | 1439 | 1439 | 0 | 0.00 | 1431.1 | 1431.1 | 139.1ms | 141.3ms | 156.6ms | 159.5ms | `200:1439` | `` |

## Notes

- Every scenario is measured once at the same fixed concurrency, default `200`, for a one-second window.
- `Successful req/s` is the useful per-second throughput under that fixed concurrency.
- This report covers public, site, admin, and common Yggdrasil client endpoints; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.
