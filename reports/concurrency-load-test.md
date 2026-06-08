# Backend Concurrency Load Test Report

- Generated at: `2026-06-08T20:20:57+08:00`
- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`
- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites
- Concurrency levels: `1,10,50,100`
- Duration per level: `2s`
- Test database: isolated `elementskin_go_test_*`, dropped by test cleanup

## Scenario Coverage

| Area | Scenario | Method | Path |
| --- | --- | --- | --- |
| Public home | `public-settings` | `GET` | `/public/settings` |
| Public home | `public-carousel` | `GET` | `/public/carousel` |
| Public library | `public-library-search` | `GET` | `/public/skin-library?limit=20&q=Load` |
| Authentication | `site-login` | `POST` | `/site-login` |
| User center | `me` | `GET` | `/me` |
| User center | `my-profiles` | `GET` | `/me/profiles?limit=20` |
| User center | `my-textures` | `GET` | `/me/textures?limit=20` |
| User center | `texture-detail` | `GET` | `/me/textures/load_texture_001_000/skin` |
| Admin console | `admin-users` | `GET` | `/admin/users?limit=20&q=Load` |
| Admin console | `admin-user-detail` | `GET` | `/admin/users/d65996432d27459282be69e32db4094e` |
| Admin console | `admin-user-profiles` | `GET` | `/admin/users/d65996432d27459282be69e32db4094e/profiles?limit=20` |
| Admin console | `admin-profiles` | `GET` | `/admin/profiles?limit=20` |
| Admin console | `admin-textures` | `GET` | `/admin/textures?limit=20` |
| Admin console | `admin-invites` | `GET` | `/admin/invites?limit=20` |
| Admin console | `admin-settings-site` | `GET` | `/admin/settings/site` |

## Capacity Summary

| Area | Scenario | Highest tested concurrency with 0% failures | Peak RPS at that level | P95 at that level |
| --- | --- | ---: | ---: | ---: |
| Public home | `public-settings` | 100 | 8864.1 | 12.9ms |
| Public home | `public-carousel` | 100 | 7679.7 | 24.5ms |
| Public library | `public-library-search` | 100 | 60703.4 | 3.1ms |
| Authentication | `site-login` | 100 | 308.1 | 472.8ms |
| User center | `me` | 100 | 21983.9 | 6.0ms |
| User center | `my-profiles` | 100 | 59958.2 | 3.1ms |
| User center | `my-textures` | 100 | 61851.6 | 3.0ms |
| User center | `texture-detail` | 100 | 34607.9 | 4.5ms |
| Admin console | `admin-users` | 100 | 58963.2 | 3.1ms |
| Admin console | `admin-user-detail` | 100 | 35789.4 | 4.3ms |
| Admin console | `admin-user-profiles` | 100 | 55466.8 | 3.4ms |
| Admin console | `admin-profiles` | 100 | 52104.9 | 3.5ms |
| Admin console | `admin-textures` | 100 | 57243.2 | 3.2ms |
| Admin console | `admin-invites` | 100 | 54489.1 | 3.4ms |
| Admin console | `admin-settings-site` | 100 | 7340.4 | 15.6ms |

## Results

| Area | Scenario | Concurrency | Requests | OK | Fail | Fail % | RPS | Avg | P50 | P95 | P99 | Status | First Error |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| Public home | `public-settings` | 1 | 2171 | 2171 | 0 | 0.00 | 1085.1 | 0.9ms | 1.1ms | 1.6ms | 1.8ms | `200:2171` | `` |
| Public home | `public-settings` | 10 | 12962 | 12962 | 0 | 0.00 | 6479.8 | 1.5ms | 1.1ms | 2.4ms | 3.0ms | `200:12962` | `` |
| Public home | `public-settings` | 50 | 15895 | 15895 | 0 | 0.00 | 7931.7 | 6.3ms | 6.1ms | 8.1ms | 9.0ms | `200:15895` | `` |
| Public home | `public-settings` | 100 | 17770 | 17770 | 0 | 0.00 | 8864.1 | 11.2ms | 11.2ms | 12.9ms | 13.8ms | `200:17770` | `` |
| Public home | `public-carousel` | 1 | 9868 | 9868 | 0 | 0.00 | 4932.5 | 0.2ms | - | 1.0ms | 1.5ms | `200:9868` | `` |
| Public home | `public-carousel` | 10 | 15388 | 15388 | 0 | 0.00 | 7689.6 | 1.3ms | 1.0ms | 2.0ms | 2.5ms | `200:15388` | `` |
| Public home | `public-carousel` | 50 | 15415 | 15415 | 0 | 0.00 | 7691.9 | 6.5ms | 6.0ms | 11.5ms | 15.3ms | `200:15415` | `` |
| Public home | `public-carousel` | 100 | 15430 | 15430 | 0 | 0.00 | 7679.7 | 12.9ms | 11.1ms | 24.5ms | 55.7ms | `200:15430` | `` |
| Public library | `public-library-search` | 1 | 13091 | 13091 | 0 | 0.00 | 6544.7 | 0.1ms | - | 0.6ms | 0.6ms | `200:13091` | `` |
| Public library | `public-library-search` | 10 | 92942 | 92942 | 0 | 0.00 | 46454.4 | 0.2ms | - | 1.0ms | 1.2ms | `200:92942` | `` |
| Public library | `public-library-search` | 50 | 113090 | 113090 | 0 | 0.00 | 56506.8 | 0.8ms | 0.7ms | 2.1ms | 3.1ms | `200:113090` | `` |
| Public library | `public-library-search` | 100 | 121542 | 121542 | 0 | 0.00 | 60703.4 | 1.6ms | 1.5ms | 3.1ms | 3.7ms | `200:121542` | `` |
| Authentication | `site-login` | 1 | 51 | 51 | 0 | 0.00 | 25.2 | 39.6ms | 39.2ms | 43.7ms | 46.1ms | `200:51` | `` |
| Authentication | `site-login` | 10 | 477 | 477 | 0 | 0.00 | 233.9 | 42.5ms | 42.2ms | 45.7ms | 49.6ms | `200:477` | `` |
| Authentication | `site-login` | 50 | 579 | 579 | 0 | 0.00 | 278.2 | 175.7ms | 162.5ms | 302.4ms | 342.1ms | `200:579` | `` |
| Authentication | `site-login` | 100 | 661 | 661 | 0 | 0.00 | 308.1 | 318.6ms | 297.0ms | 472.8ms | 690.4ms | `200:661` | `` |
| User center | `me` | 1 | 4864 | 4864 | 0 | 0.00 | 2431.7 | 0.4ms | 0.5ms | 0.6ms | 1.1ms | `200:4864` | `` |
| User center | `me` | 10 | 28756 | 28756 | 0 | 0.00 | 14372.0 | 0.7ms | 0.5ms | 1.5ms | 2.0ms | `200:28756` | `` |
| User center | `me` | 50 | 41977 | 41977 | 0 | 0.00 | 20975.3 | 2.4ms | 2.1ms | 3.7ms | 4.4ms | `200:41977` | `` |
| User center | `me` | 100 | 44018 | 44018 | 0 | 0.00 | 21983.9 | 4.5ms | 4.4ms | 6.0ms | 6.6ms | `200:44018` | `` |
| User center | `my-profiles` | 1 | 12647 | 12647 | 0 | 0.00 | 6323.5 | 0.1ms | - | 0.6ms | 0.6ms | `200:12647` | `` |
| User center | `my-profiles` | 10 | 91009 | 91009 | 0 | 0.00 | 45501.0 | 0.2ms | - | 0.7ms | 1.1ms | `200:91009` | `` |
| User center | `my-profiles` | 50 | 109236 | 109236 | 0 | 0.00 | 54596.5 | 0.9ms | 0.7ms | 2.2ms | 3.2ms | `200:109236` | `` |
| User center | `my-profiles` | 100 | 120050 | 120050 | 0 | 0.00 | 59958.2 | 1.6ms | 1.5ms | 3.1ms | 3.7ms | `200:120050` | `` |
| User center | `my-textures` | 1 | 13016 | 13016 | 0 | 0.00 | 6507.5 | 0.1ms | - | 0.5ms | 0.6ms | `200:13016` | `` |
| User center | `my-textures` | 10 | 91343 | 91343 | 0 | 0.00 | 45670.3 | 0.2ms | - | 0.8ms | 1.1ms | `200:91343` | `` |
| User center | `my-textures` | 50 | 115506 | 115506 | 0 | 0.00 | 57733.2 | 0.8ms | 0.6ms | 2.1ms | 3.0ms | `200:115506` | `` |
| User center | `my-textures` | 100 | 123765 | 123765 | 0 | 0.00 | 61851.6 | 1.6ms | 1.5ms | 3.0ms | 3.6ms | `200:123765` | `` |
| User center | `texture-detail` | 1 | 7850 | 7850 | 0 | 0.00 | 3924.9 | 0.2ms | - | 0.6ms | 0.7ms | `200:7850` | `` |
| User center | `texture-detail` | 10 | 55504 | 55504 | 0 | 0.00 | 27743.6 | 0.3ms | 0.5ms | 1.0ms | 1.5ms | `200:55504` | `` |
| User center | `texture-detail` | 50 | 72544 | 72544 | 0 | 0.00 | 36262.5 | 1.4ms | 1.2ms | 2.7ms | 3.4ms | `200:72544` | `` |
| User center | `texture-detail` | 100 | 69304 | 69304 | 0 | 0.00 | 34607.9 | 2.9ms | 2.6ms | 4.5ms | 5.1ms | `200:69304` | `` |
| Admin console | `admin-users` | 1 | 12028 | 12028 | 0 | 0.00 | 6013.2 | 0.2ms | - | 0.6ms | 0.6ms | `200:12028` | `` |
| Admin console | `admin-users` | 10 | 93432 | 93432 | 0 | 0.00 | 46706.5 | 0.2ms | - | 0.6ms | 1.1ms | `200:93432` | `` |
| Admin console | `admin-users` | 50 | 117781 | 117781 | 0 | 0.00 | 58841.1 | 0.8ms | 0.6ms | 2.0ms | 2.9ms | `200:117781` | `` |
| Admin console | `admin-users` | 100 | 118009 | 118009 | 0 | 0.00 | 58963.2 | 1.7ms | 1.5ms | 3.1ms | 3.7ms | `200:118009` | `` |
| Admin console | `admin-user-detail` | 1 | 8686 | 8686 | 0 | 0.00 | 4342.2 | 0.2ms | - | 0.6ms | 0.6ms | `200:8686` | `` |
| Admin console | `admin-user-detail` | 10 | 53888 | 53888 | 0 | 0.00 | 26935.1 | 0.4ms | 0.5ms | 1.0ms | 1.5ms | `200:53888` | `` |
| Admin console | `admin-user-detail` | 50 | 71075 | 71075 | 0 | 0.00 | 35521.2 | 1.4ms | 1.2ms | 2.7ms | 3.5ms | `200:71075` | `` |
| Admin console | `admin-user-detail` | 100 | 71645 | 71645 | 0 | 0.00 | 35789.4 | 2.8ms | 2.6ms | 4.3ms | 4.8ms | `200:71645` | `` |
| Admin console | `admin-user-profiles` | 1 | 12297 | 12297 | 0 | 0.00 | 6147.8 | 0.2ms | - | 0.6ms | 0.6ms | `200:12297` | `` |
| Admin console | `admin-user-profiles` | 10 | 89845 | 89845 | 0 | 0.00 | 44922.0 | 0.2ms | - | 0.6ms | 1.1ms | `200:89845` | `` |
| Admin console | `admin-user-profiles` | 50 | 115685 | 115685 | 0 | 0.00 | 57819.1 | 0.8ms | 0.6ms | 2.0ms | 2.9ms | `200:115685` | `` |
| Admin console | `admin-user-profiles` | 100 | 111016 | 111016 | 0 | 0.00 | 55466.8 | 1.8ms | 1.5ms | 3.4ms | 4.1ms | `200:111016` | `` |
| Admin console | `admin-profiles` | 1 | 12511 | 12511 | 0 | 0.00 | 6254.8 | 0.1ms | - | 0.6ms | 0.6ms | `200:12511` | `` |
| Admin console | `admin-profiles` | 10 | 93316 | 93316 | 0 | 0.00 | 46646.0 | 0.2ms | - | 0.8ms | 1.1ms | `200:93316` | `` |
| Admin console | `admin-profiles` | 50 | 108883 | 108883 | 0 | 0.00 | 54417.7 | 0.9ms | 0.7ms | 2.1ms | 3.2ms | `200:108883` | `` |
| Admin console | `admin-profiles` | 100 | 104308 | 104308 | 0 | 0.00 | 52104.9 | 1.9ms | 1.7ms | 3.5ms | 4.2ms | `200:104308` | `` |
| Admin console | `admin-textures` | 1 | 12634 | 12634 | 0 | 0.00 | 6315.6 | 0.1ms | - | 0.6ms | 0.6ms | `200:12634` | `` |
| Admin console | `admin-textures` | 10 | 91933 | 91933 | 0 | 0.00 | 45963.5 | 0.2ms | - | 0.6ms | 1.1ms | `200:91933` | `` |
| Admin console | `admin-textures` | 50 | 113085 | 113085 | 0 | 0.00 | 56534.8 | 0.8ms | 0.7ms | 2.1ms | 3.0ms | `200:113085` | `` |
| Admin console | `admin-textures` | 100 | 114620 | 114620 | 0 | 0.00 | 57243.2 | 1.7ms | 1.5ms | 3.2ms | 3.8ms | `200:114620` | `` |
| Admin console | `admin-invites` | 1 | 12792 | 12792 | 0 | 0.00 | 6395.7 | 0.1ms | - | 0.6ms | 0.6ms | `200:12792` | `` |
| Admin console | `admin-invites` | 10 | 92894 | 92894 | 0 | 0.00 | 46435.6 | 0.2ms | - | 0.7ms | 1.1ms | `200:92894` | `` |
| Admin console | `admin-invites` | 50 | 108981 | 108981 | 0 | 0.00 | 54474.1 | 0.9ms | 0.7ms | 2.2ms | 3.2ms | `200:108981` | `` |
| Admin console | `admin-invites` | 100 | 109031 | 109031 | 0 | 0.00 | 54489.1 | 1.8ms | 1.6ms | 3.4ms | 4.0ms | `200:109031` | `` |
| Admin console | `admin-settings-site` | 1 | 1906 | 1906 | 0 | 0.00 | 952.9 | 1.0ms | 1.1ms | 1.6ms | 1.8ms | `200:1906` | `` |
| Admin console | `admin-settings-site` | 10 | 12068 | 12068 | 0 | 0.00 | 6031.1 | 1.6ms | 1.6ms | 2.6ms | 3.0ms | `200:12068` | `` |
| Admin console | `admin-settings-site` | 50 | 14303 | 14303 | 0 | 0.00 | 7135.6 | 7.0ms | 7.0ms | 8.6ms | 9.2ms | `200:14303` | `` |
| Admin console | `admin-settings-site` | 100 | 14728 | 14728 | 0 | 0.00 | 7340.4 | 13.6ms | 13.5ms | 15.6ms | 16.6ms | `200:14728` | `` |

## Notes

- This report focuses on realistic frontend page-load endpoints and login; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.
