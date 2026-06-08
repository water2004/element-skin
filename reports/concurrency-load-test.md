# Backend Concurrency Load Test Report

- Generated at: `2026-06-09T03:51:56+08:00`
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
| Public home | `public-carousel` | `GET` | `/public/carousel` |
| Public library | `public-library-search` | `GET` | `/public/skin-library?limit=20&q=Load` |
| Authentication | `site-login` | `POST` | `/site-login` |
| Yggdrasil | `ygg-metadata` | `GET` | `/` |
| Yggdrasil | `ygg-authenticate` | `POST` | `/authserver/authenticate` |
| Yggdrasil | `ygg-validate` | `POST` | `/authserver/validate` |
| Yggdrasil | `ygg-profile` | `GET` | `/sessionserver/session/minecraft/profile/6dc1ada33df1405e9182ee25af693298` |
| Yggdrasil | `ygg-lookup-name` | `GET` | `/api/users/profiles/minecraft/LoadProfile002_0` |
| Yggdrasil | `ygg-has-joined` | `GET` | `/sessionserver/session/minecraft/hasJoined?username=LoadProfile002_0&serverId=load_ygg_server` |
| User center | `me` | `GET` | `/me` |
| User center | `my-profiles` | `GET` | `/me/profiles?limit=20` |
| User center | `my-textures` | `GET` | `/me/textures?limit=20` |
| User center | `texture-detail` | `GET` | `/me/textures/load_texture_001_000/skin` |
| Admin console | `admin-users` | `GET` | `/admin/users?limit=20&q=Load` |
| Admin console | `admin-user-detail` | `GET` | `/admin/users/3da8767ae1014629af29fe794f865787` |
| Admin console | `admin-user-profiles` | `GET` | `/admin/users/3da8767ae1014629af29fe794f865787/profiles?limit=20` |
| Admin console | `admin-profiles` | `GET` | `/admin/profiles?limit=20` |
| Admin console | `admin-textures` | `GET` | `/admin/textures?limit=20` |
| Admin console | `admin-invites` | `GET` | `/admin/invites?limit=20` |
| Admin console | `admin-settings-site` | `GET` | `/admin/settings/site` |

## Fixed-200 One-Second Results

| Area | Scenario | Concurrency | Requests | OK | Fail | Fail % | Successful req/s | Total req/s | Avg | P50 | P95 | P99 | Status | First Error |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| Public home | `public-settings` | 200 | 26259 | 26259 | 0 | 0.00 | 26105.8 | 26105.8 | 7.6ms | 6.2ms | 9.1ms | 13.0ms | `200:26259` | `` |
| Public home | `public-carousel` | 200 | 30569 | 30569 | 0 | 0.00 | 30420.8 | 30420.8 | 6.5ms | 6.2ms | 8.2ms | 9.5ms | `200:30569` | `` |
| Public library | `public-library-search` | 200 | 17082 | 17082 | 0 | 0.00 | 16894.7 | 16894.7 | 11.6ms | 11.3ms | 17.0ms | 36.0ms | `200:17082` | `` |
| Authentication | `site-login` | 200 | 387 | 387 | 0 | 0.00 | 305.6 | 305.6 | 612.3ms | 598.0ms | 695.7ms | 1.13s | `200:387` | `` |
| Yggdrasil | `ygg-metadata` | 200 | 33094 | 33094 | 0 | 0.00 | 32938.5 | 32938.5 | 6.0ms | 5.8ms | 7.5ms | 8.5ms | `200:33094` | `` |
| Yggdrasil | `ygg-authenticate` | 200 | 330 | 330 | 0 | 0.00 | 292.1 | 292.1 | 637.1ms | 709.6ms | 1.04s | 1.05s | `200:330` | `` |
| Yggdrasil | `ygg-validate` | 200 | 31956 | 31956 | 0 | 0.00 | 31803.1 | 31803.1 | 6.3ms | 6.0ms | 7.8ms | 8.4ms | `204:31956` | `` |
| Yggdrasil | `ygg-profile` | 200 | 61498 | 61498 | 0 | 0.00 | 61355.0 | 61355.0 | 3.2ms | 3.0ms | 5.2ms | 6.4ms | `200:61498` | `` |
| Yggdrasil | `ygg-lookup-name` | 200 | 65119 | 65119 | 0 | 0.00 | 64973.6 | 64973.6 | 3.0ms | 2.8ms | 4.8ms | 6.3ms | `200:65119` | `` |
| Yggdrasil | `ygg-has-joined` | 200 | 2260 | 2260 | 0 | 0.00 | 2072.2 | 2072.2 | 92.3ms | 91.2ms | 127.6ms | 159.8ms | `200:2260` | `` |
| User center | `me` | 200 | 20429 | 20429 | 0 | 0.00 | 20258.1 | 20258.1 | 9.8ms | 9.5ms | 13.6ms | 20.0ms | `200:20429` | `` |
| User center | `my-profiles` | 200 | 29087 | 29087 | 0 | 0.00 | 28928.8 | 28928.8 | 6.9ms | 6.6ms | 8.9ms | 11.1ms | `200:29087` | `` |
| User center | `my-textures` | 200 | 29991 | 29991 | 0 | 0.00 | 29838.0 | 29838.0 | 6.6ms | 6.4ms | 8.5ms | 10.2ms | `200:29991` | `` |
| User center | `texture-detail` | 200 | 29373 | 29373 | 0 | 0.00 | 29216.8 | 29216.8 | 6.8ms | 6.5ms | 8.6ms | 9.7ms | `200:29373` | `` |
| Admin console | `admin-users` | 200 | 18465 | 18465 | 0 | 0.00 | 18290.2 | 18290.2 | 10.7ms | 10.6ms | 16.7ms | 19.3ms | `200:18465` | `` |
| Admin console | `admin-user-detail` | 200 | 29028 | 29028 | 0 | 0.00 | 28837.8 | 28837.8 | 6.9ms | 6.6ms | 8.9ms | 10.6ms | `200:29028` | `` |
| Admin console | `admin-user-profiles` | 200 | 28903 | 28903 | 0 | 0.00 | 28739.6 | 28739.6 | 6.9ms | 6.6ms | 9.1ms | 10.9ms | `200:28903` | `` |
| Admin console | `admin-profiles` | 200 | 22809 | 22809 | 0 | 0.00 | 22630.1 | 22630.1 | 8.7ms | 8.5ms | 13.2ms | 15.8ms | `200:22809` | `` |
| Admin console | `admin-textures` | 200 | 22978 | 22978 | 0 | 0.00 | 22827.7 | 22827.7 | 8.6ms | 8.4ms | 13.6ms | 16.4ms | `200:22978` | `` |
| Admin console | `admin-invites` | 200 | 24721 | 24721 | 0 | 0.00 | 24581.6 | 24581.6 | 8.0ms | 7.9ms | 12.1ms | 14.3ms | `200:24721` | `` |
| Admin console | `admin-settings-site` | 200 | 2600 | 2600 | 0 | 0.00 | 2415.1 | 2415.1 | 82.5ms | 81.4ms | 90.0ms | 91.9ms | `200:2600` | `` |

## Notes

- Every scenario is measured once at the same fixed concurrency, default `200`, for a one-second window.
- `Successful req/s` is the useful per-second throughput under that fixed concurrency.
- This report covers public, site, admin, and common Yggdrasil client endpoints; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.
