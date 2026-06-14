# Python Backend Concurrency Load Test Report

- Generated at: `2026-06-15T02:25:34+0800`
- Harness: `LOADTEST_ENABLE=1 pytest tests/loadtest/test_backend_load.py -q -s`
- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites, 1 pre-joined Yggdrasil session
- Fixed concurrency: `200`
- Duration per level: `1s`
- Backend database pool used by harness: `20` max connections
- Test database: isolated `elementskin_py_test_*`, dropped by test cleanup
- HTTP server: real local Uvicorn server, closed by test cleanup
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
| Yggdrasil | `ygg-profile` | `GET` | `/sessionserver/session/minecraft/profile/ea68b70113fa4ad4a63865d22e26b469` |
| Yggdrasil | `ygg-lookup-name` | `GET` | `/api/users/profiles/minecraft/LoadProfile002_0` |
| Yggdrasil | `ygg-has-joined` | `GET` | `/sessionserver/session/minecraft/hasJoined?username=LoadProfile002_0&serverId=load_ygg_server` |
| User center | `me` | `GET` | `/me` |
| User center | `my-profiles` | `GET` | `/me/profiles?limit=20` |
| User center | `my-textures` | `GET` | `/me/textures?limit=20` |
| User center | `texture-detail` | `GET` | `/me/textures/load_texture_001_000/skin` |
| Admin console | `admin-users` | `GET` | `/admin/users?limit=20&q=Load` |
| Admin console | `admin-user-detail` | `GET` | `/admin/users/b407680622df4c298f57b386e3c659fb` |
| Admin console | `admin-user-profiles` | `GET` | `/admin/users/b407680622df4c298f57b386e3c659fb/profiles?limit=20` |
| Admin console | `admin-profiles` | `GET` | `/admin/profiles?limit=20` |
| Admin console | `admin-textures` | `GET` | `/admin/textures?limit=20` |
| Admin console | `admin-invites` | `GET` | `/admin/invites?limit=20` |
| Admin console | `admin-settings-site` | `GET` | `/admin/settings/site` |

## Fixed-200 One-Second Results

| Area | Scenario | Concurrency | Requests | OK | Fail | Fail % | Successful req/s | Total req/s | Avg | P50 | P95 | P99 | Status | First Error |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| Public home | `public-settings` | 200 | 2000 | 2000 | 0 | 0.00 | 1913.7 | 1913.7 | 102.6ms | 88.5ms | 200.3ms | 213.6ms | `200:2000` | `` |
| Public home | `public-carousel` | 200 | 2200 | 2200 | 0 | 0.00 | 2138.0 | 2138.0 | 92.0ms | 81.7ms | 113.4ms | 114.4ms | `200:2200` | `` |
| Public library | `public-library-search` | 200 | 866 | 866 | 0 | 0.00 | 777.9 | 777.9 | 242.0ms | 216.1ms | 552.6ms | 714.3ms | `200:866` | `` |
| Authentication | `site-login` | 200 | 232 | 232 | 0 | 0.00 | 42.1 | 42.1 | 2.84s | 2.94s | 4.58s | 4.76s | `200:232` | `` |
| Yggdrasil | `ygg-metadata` | 200 | 2800 | 2800 | 0 | 0.00 | 2694.4 | 2694.4 | 73.1ms | 60.4ms | 110.9ms | 123.9ms | `200:2800` | `` |
| Yggdrasil | `ygg-authenticate` | 200 | 232 | 232 | 0 | 0.00 | 42.6 | 42.6 | 2.80s | 2.88s | 4.54s | 4.70s | `200:232` | `` |
| Yggdrasil | `ygg-validate` | 200 | 1185 | 1185 | 0 | 0.00 | 1126.3 | 1126.3 | 171.3ms | 150.1ms | 422.1ms | 655.2ms | `204:1185` | `` |
| Yggdrasil | `ygg-profile` | 200 | 1880 | 1880 | 0 | 0.00 | 1782.7 | 1782.7 | 108.3ms | 97.4ms | 151.1ms | 191.4ms | `200:1880` | `` |
| Yggdrasil | `ygg-lookup-name` | 200 | 1920 | 1920 | 0 | 0.00 | 1827.5 | 1827.5 | 105.4ms | 95.0ms | 164.2ms | 202.9ms | `200:1920` | `` |
| Yggdrasil | `ygg-has-joined` | 200 | 415 | 415 | 0 | 0.00 | 250.8 | 250.8 | 632.6ms | 589.5ms | 1.36s | 1.55s | `200:415` | `` |
| User center | `me` | 200 | 1086 | 1086 | 0 | 0.00 | 984.3 | 984.3 | 192.3ms | 176.1ms | 384.1ms | 497.4ms | `200:1086` | `` |
| User center | `my-profiles` | 200 | 956 | 956 | 0 | 0.00 | 891.2 | 891.2 | 207.2ms | 172.0ms | 469.3ms | 620.5ms | `200:956` | `` |
| User center | `my-textures` | 200 | 1243 | 1243 | 0 | 0.00 | 1125.8 | 1125.8 | 169.0ms | 152.8ms | 361.6ms | 543.8ms | `200:1243` | `` |
| User center | `texture-detail` | 200 | 1173 | 1173 | 0 | 0.00 | 1101.1 | 1101.1 | 166.6ms | 140.2ms | 360.5ms | 483.6ms | `200:1173` | `` |
| Admin console | `admin-users` | 200 | 778 | 778 | 0 | 0.00 | 672.9 | 672.9 | 272.9ms | 201.6ms | 780.4ms | 941.2ms | `200:778` | `` |
| Admin console | `admin-user-detail` | 200 | 916 | 916 | 0 | 0.00 | 822.2 | 822.2 | 230.5ms | 194.7ms | 510.3ms | 689.8ms | `200:916` | `` |
| Admin console | `admin-user-profiles` | 200 | 1122 | 1122 | 0 | 0.00 | 1032.5 | 1032.5 | 185.6ms | 118.4ms | 689.5ms | 853.1ms | `200:1122` | `` |
| Admin console | `admin-profiles` | 200 | 919 | 919 | 0 | 0.00 | 809.2 | 809.2 | 230.7ms | 154.6ms | 822.5ms | 912.1ms | `200:919` | `` |
| Admin console | `admin-textures` | 200 | 905 | 905 | 0 | 0.00 | 793.0 | 793.0 | 235.3ms | 191.9ms | 659.7ms | 964.9ms | `200:905` | `` |
| Admin console | `admin-invites` | 200 | 1039 | 1039 | 0 | 0.00 | 915.9 | 915.9 | 204.3ms | 177.8ms | 371.8ms | 505.6ms | `200:1039` | `` |
| Admin console | `admin-settings-site` | 200 | 1400 | 1400 | 0 | 0.00 | 1318.3 | 1318.3 | 143.9ms | 39.3ms | 890.1ms | 1.03s | `200:1400` | `` |

## Notes

- Every scenario is measured once at the same fixed concurrency, default `200`, for a one-second window.
- `Successful req/s` is the useful per-second throughput under that fixed concurrency.
- This report covers public, site, admin, and common Yggdrasil client endpoints; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the local HTTP server, closes the database pool, terminates leftover database sessions, and drops the temporary PostgreSQL database during cleanup.
