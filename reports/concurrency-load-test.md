# Backend Concurrency Load Test Report

- Generated at: `2026-06-09T00:03:48+08:00`
- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`
- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites
- Concurrency search levels: `1,10,50,100,200,400,800`
- Duration per level: `1s`
- Pass condition: failure rate <= `1.00%`, p95 <= `1s`
- Backend database pool used by harness: `20` max connections
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
| Admin console | `admin-user-detail` | `GET` | `/admin/users/f9ae9d24567f43eca5a9fa739bce184f` |
| Admin console | `admin-user-profiles` | `GET` | `/admin/users/f9ae9d24567f43eca5a9fa739bce184f/profiles?limit=20` |
| Admin console | `admin-profiles` | `GET` | `/admin/profiles?limit=20` |
| Admin console | `admin-textures` | `GET` | `/admin/textures?limit=20` |
| Admin console | `admin-invites` | `GET` | `/admin/invites?limit=20` |
| Admin console | `admin-settings-site` | `GET` | `/admin/settings/site` |

## Per-Endpoint One-Second Capacity

| Area | Scenario | Sustainable concurrency | Successful req/s at that point | Total req/s | P95 | P99 | Tested ceiling? |
| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |
| Public home | `public-settings` | 200 | 10548.9 | 10548.9 | 21.2ms | 29.9ms | no |
| Public home | `public-carousel` | 200 | 7160.1 | 7160.1 | 36.5ms | 257.0ms | no |
| Public library | `public-library-search` | 200 | 15493.4 | 15493.4 | 16.9ms | 22.1ms | no |
| Authentication | `site-login` | 100 | 273.0 | 273.0 | 554.9ms | 633.2ms | no |
| User center | `me` | 200 | 23020.0 | 23020.0 | 11.1ms | 13.3ms | no |
| User center | `my-profiles` | 200 | 34708.1 | 34708.1 | 8.0ms | 9.5ms | no |
| User center | `my-textures` | 200 | 34405.6 | 34405.6 | 8.0ms | 9.2ms | no |
| User center | `texture-detail` | 200 | 34010.7 | 34010.7 | 8.6ms | 10.7ms | no |
| Admin console | `admin-users` | 200 | 17035.2 | 17035.2 | 14.6ms | 20.9ms | no |
| Admin console | `admin-user-detail` | 200 | 37780.0 | 37780.0 | 7.5ms | 8.8ms | no |
| Admin console | `admin-user-profiles` | 200 | 35612.0 | 35612.0 | 7.9ms | 9.2ms | no |
| Admin console | `admin-profiles` | 200 | 22232.1 | 22232.1 | 10.9ms | 14.3ms | no |
| Admin console | `admin-textures` | 200 | 19392.6 | 19392.6 | 12.8ms | 17.7ms | no |
| Admin console | `admin-invites` | 200 | 21264.5 | 21264.5 | 12.1ms | 16.4ms | no |
| Admin console | `admin-settings-site` | 200 | 8377.5 | 8377.5 | 29.1ms | 46.9ms | no |

## Results

| Area | Scenario | Concurrency | Requests | OK | Fail | Fail % | Successful req/s | Total req/s | Avg | P50 | P95 | P99 | Pass | Status | First Error |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- | --- |
| Public home | `public-settings` | 1 | 964 | 964 | 0 | 0.00 | 963.5 | 963.5 | 1.0ms | 1.1ms | 1.7ms | 2.1ms | yes | `200:964` | `` |
| Public home | `public-settings` | 10 | 5320 | 5320 | 0 | 0.00 | 5315.8 | 5315.8 | 1.9ms | 1.5ms | 2.3ms | 2.8ms | yes | `200:5320` | `` |
| Public home | `public-settings` | 50 | 8860 | 8860 | 0 | 0.00 | 8840.7 | 8840.7 | 5.6ms | 5.0ms | 8.0ms | 9.7ms | yes | `200:8860` | `` |
| Public home | `public-settings` | 100 | 10597 | 10597 | 0 | 0.00 | 10533.9 | 10533.9 | 9.4ms | 9.2ms | 11.6ms | 13.2ms | yes | `200:10597` | `` |
| Public home | `public-settings` | 200 | 10664 | 10664 | 0 | 0.00 | 10548.9 | 10548.9 | 18.8ms | 18.7ms | 21.2ms | 29.9ms | yes | `200:10664` | `` |
| Public home | `public-settings` | 400 | 26789 | 4840 | 21949 | 81.93 | 4717.4 | 26110.7 | 15.1ms | 3.5ms | 47.9ms | 174.6ms | no | `200:4840` | `Get "http://127.0.0.1:49283/public/settings": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| Public home | `public-carousel` | 1 | 3714 | 3714 | 0 | 0.00 | 3711.9 | 3711.9 | 0.3ms | - | 1.0ms | 1.5ms | yes | `200:3714` | `` |
| Public home | `public-carousel` | 10 | 8645 | 8645 | 0 | 0.00 | 8639.6 | 8639.6 | 1.1ms | 1.0ms | 2.0ms | 2.5ms | yes | `200:8645` | `` |
| Public home | `public-carousel` | 50 | 8404 | 8404 | 0 | 0.00 | 8385.9 | 8385.9 | 5.9ms | 5.4ms | 11.8ms | 16.0ms | yes | `200:8404` | `` |
| Public home | `public-carousel` | 100 | 7988 | 7988 | 0 | 0.00 | 7911.7 | 7911.7 | 12.5ms | 10.5ms | 24.0ms | 55.5ms | yes | `200:7988` | `` |
| Public home | `public-carousel` | 200 | 7354 | 7354 | 0 | 0.00 | 7160.1 | 7160.1 | 27.5ms | 20.8ms | 36.5ms | 257.0ms | yes | `200:7354` | `` |
| Public home | `public-carousel` | 400 | 25580 | 5984 | 19596 | 76.61 | 5699.3 | 24362.8 | 16.1ms | 3.7ms | 63.2ms | 159.6ms | no | `200:5984` | `Get "http://127.0.0.1:49283/public/carousel": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| Public library | `public-library-search` | 1 | 1622 | 1622 | 0 | 0.00 | 1621.1 | 1621.1 | 0.6ms | 0.6ms | 1.0ms | 1.3ms | yes | `200:1622` | `` |
| Public library | `public-library-search` | 10 | 12755 | 12755 | 0 | 0.00 | 12746.3 | 12746.3 | 0.8ms | 0.6ms | 1.5ms | 2.0ms | yes | `200:12755` | `` |
| Public library | `public-library-search` | 50 | 15955 | 15955 | 0 | 0.00 | 15920.9 | 15920.9 | 3.0ms | 2.5ms | 6.5ms | 8.6ms | yes | `200:15955` | `` |
| Public library | `public-library-search` | 100 | 15041 | 15041 | 0 | 0.00 | 14942.4 | 14942.4 | 6.4ms | 6.3ms | 9.7ms | 12.8ms | yes | `200:15041` | `` |
| Public library | `public-library-search` | 200 | 15653 | 15653 | 0 | 0.00 | 15493.4 | 15493.4 | 12.4ms | 12.2ms | 16.9ms | 22.1ms | yes | `200:15653` | `` |
| Public library | `public-library-search` | 400 | 28236 | 10743 | 17493 | 61.95 | 10565.1 | 27768.3 | 14.1ms | 4.6ms | 26.7ms | 163.1ms | no | `200:10743` | `Get "http://127.0.0.1:49283/public/skin-library?limit=20&q=Load": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| Authentication | `site-login` | 1 | 25 | 25 | 0 | 0.00 | 24.2 | 24.2 | 41.3ms | 40.5ms | 45.4ms | 48.0ms | yes | `200:25` | `` |
| Authentication | `site-login` | 10 | 240 | 240 | 0 | 0.00 | 230.9 | 230.9 | 42.8ms | 42.7ms | 45.2ms | 48.1ms | yes | `200:240` | `` |
| Authentication | `site-login` | 50 | 299 | 299 | 0 | 0.00 | 280.6 | 280.6 | 171.7ms | 138.4ms | 473.8ms | 685.1ms | yes | `200:299` | `` |
| Authentication | `site-login` | 100 | 346 | 346 | 0 | 0.00 | 273.0 | 273.0 | 349.9ms | 331.6ms | 554.9ms | 633.2ms | yes | `200:346` | `` |
| Authentication | `site-login` | 200 | 333 | 333 | 0 | 0.00 | 287.8 | 287.8 | 657.7ms | 704.8ms | 1.10s | 1.12s | no | `200:333` | `` |
| User center | `me` | 1 | 2226 | 2226 | 0 | 0.00 | 2224.9 | 2224.9 | 0.4ms | 0.5ms | 0.7ms | 1.1ms | yes | `200:2226` | `` |
| User center | `me` | 10 | 19494 | 19494 | 0 | 0.00 | 19465.3 | 19465.3 | 0.5ms | 0.5ms | 1.5ms | 2.0ms | yes | `200:19494` | `` |
| User center | `me` | 50 | 21667 | 21667 | 0 | 0.00 | 21634.6 | 21634.6 | 2.2ms | 2.0ms | 4.8ms | 6.6ms | yes | `200:21667` | `` |
| User center | `me` | 100 | 21000 | 21000 | 0 | 0.00 | 20944.5 | 20944.5 | 4.7ms | 4.2ms | 8.1ms | 10.9ms | yes | `200:21000` | `` |
| User center | `me` | 200 | 23181 | 23181 | 0 | 0.00 | 23020.0 | 23020.0 | 8.6ms | 8.1ms | 11.1ms | 13.3ms | yes | `200:23181` | `` |
| User center | `me` | 400 | 32994 | 12812 | 20182 | 61.17 | 12660.7 | 32604.4 | 12.1ms | 4.2ms | 23.1ms | 97.3ms | no | `200:12812` | `Get "http://127.0.0.1:49283/me": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| User center | `my-profiles` | 1 | 3375 | 3375 | 0 | 0.00 | 3373.2 | 3373.2 | 0.3ms | 0.5ms | 0.6ms | 0.7ms | yes | `200:3375` | `` |
| User center | `my-profiles` | 10 | 28914 | 28914 | 0 | 0.00 | 28913.5 | 28913.5 | 0.3ms | - | 1.0ms | 1.5ms | yes | `200:28914` | `` |
| User center | `my-profiles` | 50 | 32664 | 32664 | 0 | 0.00 | 32620.2 | 32620.2 | 1.5ms | 1.1ms | 3.9ms | 5.6ms | yes | `200:32664` | `` |
| User center | `my-profiles` | 100 | 34381 | 34381 | 0 | 0.00 | 34334.6 | 34334.6 | 2.8ms | 2.5ms | 5.4ms | 7.2ms | yes | `200:34381` | `` |
| User center | `my-profiles` | 200 | 34856 | 34856 | 0 | 0.00 | 34708.1 | 34708.1 | 5.7ms | 5.2ms | 8.0ms | 9.5ms | yes | `200:34856` | `` |
| User center | `my-profiles` | 400 | 37472 | 19340 | 18132 | 48.39 | 19184.3 | 37170.3 | 10.6ms | 8.5ms | 18.3ms | 59.4ms | no | `200:19340` | `Get "http://127.0.0.1:49283/me/profiles?limit=20": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| User center | `my-textures` | 1 | 3111 | 3111 | 0 | 0.00 | 3110.6 | 3110.6 | 0.3ms | 0.5ms | 0.6ms | 0.7ms | yes | `200:3111` | `` |
| User center | `my-textures` | 10 | 26231 | 26231 | 0 | 0.00 | 26222.2 | 26222.2 | 0.4ms | 0.5ms | 1.0ms | 1.6ms | yes | `200:26231` | `` |
| User center | `my-textures` | 50 | 33026 | 33026 | 0 | 0.00 | 32999.9 | 32999.9 | 1.4ms | 1.1ms | 3.8ms | 5.5ms | yes | `200:33026` | `` |
| User center | `my-textures` | 100 | 32999 | 32999 | 0 | 0.00 | 32929.8 | 32929.8 | 3.0ms | 2.5ms | 5.7ms | 7.3ms | yes | `200:32999` | `` |
| User center | `my-textures` | 200 | 34531 | 34531 | 0 | 0.00 | 34405.6 | 34405.6 | 5.7ms | 5.4ms | 8.0ms | 9.2ms | yes | `200:34531` | `` |
| User center | `my-textures` | 400 | 36088 | 17802 | 18286 | 50.67 | 17650.7 | 35781.3 | 11.0ms | 7.0ms | 19.5ms | 63.7ms | no | `200:17802` | `Get "http://127.0.0.1:49283/me/textures?limit=20": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| User center | `texture-detail` | 1 | 3620 | 3620 | 0 | 0.00 | 3618.4 | 3618.4 | 0.3ms | - | 0.6ms | 0.7ms | yes | `200:3620` | `` |
| User center | `texture-detail` | 10 | 29242 | 29242 | 0 | 0.00 | 29240.0 | 29240.0 | 0.3ms | - | 1.0ms | 1.5ms | yes | `200:29242` | `` |
| User center | `texture-detail` | 50 | 36128 | 36128 | 0 | 0.00 | 36103.8 | 36103.8 | 1.3ms | 1.0ms | 3.4ms | 5.2ms | yes | `200:36128` | `` |
| User center | `texture-detail` | 100 | 36853 | 36853 | 0 | 0.00 | 36809.2 | 36809.2 | 2.6ms | 2.3ms | 5.1ms | 6.8ms | yes | `200:36853` | `` |
| User center | `texture-detail` | 200 | 34121 | 34121 | 0 | 0.00 | 34010.7 | 34010.7 | 5.8ms | 5.4ms | 8.6ms | 10.7ms | yes | `200:34121` | `` |
| User center | `texture-detail` | 400 | 37479 | 19006 | 18473 | 49.29 | 18886.1 | 37242.6 | 10.6ms | 7.5ms | 17.0ms | 47.8ms | no | `200:19006` | `Get "http://127.0.0.1:49283/me/textures/load_texture_001_000/skin": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-users` | 1 | 1985 | 1985 | 0 | 0.00 | 1984.7 | 1984.7 | 0.5ms | 0.5ms | 1.0ms | 1.2ms | yes | `200:1985` | `` |
| Admin console | `admin-users` | 10 | 14693 | 14693 | 0 | 0.00 | 14686.0 | 14686.0 | 0.6ms | 0.5ms | 1.3ms | 1.7ms | yes | `200:14693` | `` |
| Admin console | `admin-users` | 50 | 17850 | 17850 | 0 | 0.00 | 17817.9 | 17817.9 | 2.6ms | 2.1ms | 6.0ms | 7.8ms | yes | `200:17850` | `` |
| Admin console | `admin-users` | 100 | 16445 | 16445 | 0 | 0.00 | 16367.7 | 16367.7 | 5.9ms | 5.8ms | 8.9ms | 11.8ms | yes | `200:16445` | `` |
| Admin console | `admin-users` | 200 | 17160 | 17160 | 0 | 0.00 | 17035.2 | 17035.2 | 11.5ms | 11.4ms | 14.6ms | 20.9ms | yes | `200:17160` | `` |
| Admin console | `admin-users` | 400 | 29315 | 11440 | 17875 | 60.98 | 11221.3 | 28754.5 | 13.6ms | 4.0ms | 27.7ms | 142.0ms | no | `200:11440` | `Get "http://127.0.0.1:49283/admin/users?limit=20&q=Load": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-user-detail` | 1 | 3496 | 3496 | 0 | 0.00 | 3494.2 | 3494.2 | 0.3ms | - | 0.6ms | 0.7ms | yes | `200:3496` | `` |
| Admin console | `admin-user-detail` | 10 | 32195 | 32195 | 0 | 0.00 | 32180.6 | 32180.6 | 0.3ms | - | 1.0ms | 1.5ms | yes | `200:32195` | `` |
| Admin console | `admin-user-detail` | 50 | 34220 | 34220 | 0 | 0.00 | 34165.9 | 34165.9 | 1.4ms | 1.0ms | 3.5ms | 5.5ms | yes | `200:34220` | `` |
| Admin console | `admin-user-detail` | 100 | 33729 | 33729 | 0 | 0.00 | 33657.5 | 33657.5 | 2.9ms | 2.5ms | 5.5ms | 7.5ms | yes | `200:33729` | `` |
| Admin console | `admin-user-detail` | 200 | 37927 | 37927 | 0 | 0.00 | 37780.0 | 37780.0 | 5.2ms | 4.8ms | 7.5ms | 8.8ms | yes | `200:37927` | `` |
| Admin console | `admin-user-detail` | 400 | 39454 | 22488 | 16966 | 43.00 | 22335.2 | 39186.0 | 10.1ms | 8.9ms | 16.1ms | 48.6ms | no | `200:22488` | `Get "http://127.0.0.1:49283/admin/users/f9ae9d24567f43eca5a9fa739bce184f": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-user-profiles` | 1 | 3351 | 3351 | 0 | 0.00 | 3350.0 | 3350.0 | 0.3ms | 0.5ms | 0.6ms | 0.7ms | yes | `200:3351` | `` |
| Admin console | `admin-user-profiles` | 10 | 28821 | 28821 | 0 | 0.00 | 28794.1 | 28794.1 | 0.3ms | - | 1.0ms | 1.5ms | yes | `200:28821` | `` |
| Admin console | `admin-user-profiles` | 50 | 32181 | 32181 | 0 | 0.00 | 32125.8 | 32125.8 | 1.5ms | 1.1ms | 3.8ms | 5.6ms | yes | `200:32181` | `` |
| Admin console | `admin-user-profiles` | 100 | 35052 | 35052 | 0 | 0.00 | 34991.4 | 34991.4 | 2.8ms | 2.5ms | 5.4ms | 6.9ms | yes | `200:35052` | `` |
| Admin console | `admin-user-profiles` | 200 | 35729 | 35729 | 0 | 0.00 | 35612.0 | 35612.0 | 5.5ms | 5.1ms | 7.9ms | 9.2ms | yes | `200:35729` | `` |
| Admin console | `admin-user-profiles` | 400 | 38623 | 24036 | 14587 | 37.77 | 23878.9 | 38370.6 | 10.3ms | 9.1ms | 16.1ms | 44.4ms | no | `200:24036` | `Get "http://127.0.0.1:49283/admin/users/f9ae9d24567f43eca5a9fa739bce184f/profiles?limit=20": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-profiles` | 1 | 2391 | 2391 | 0 | 0.00 | 2390.6 | 2390.6 | 0.4ms | 0.5ms | 0.7ms | 1.1ms | yes | `200:2391` | `` |
| Admin console | `admin-profiles` | 10 | 17040 | 17040 | 0 | 0.00 | 17032.9 | 17032.9 | 0.6ms | 0.5ms | 1.5ms | 2.0ms | yes | `200:17040` | `` |
| Admin console | `admin-profiles` | 50 | 21947 | 21947 | 0 | 0.00 | 21929.0 | 21929.0 | 2.1ms | 1.7ms | 5.2ms | 6.7ms | yes | `200:21947` | `` |
| Admin console | `admin-profiles` | 100 | 21731 | 21731 | 0 | 0.00 | 21679.0 | 21679.0 | 4.5ms | 4.4ms | 7.2ms | 9.2ms | yes | `200:21731` | `` |
| Admin console | `admin-profiles` | 200 | 22424 | 22424 | 0 | 0.00 | 22232.1 | 22232.1 | 8.8ms | 8.9ms | 10.9ms | 14.3ms | yes | `200:22424` | `` |
| Admin console | `admin-profiles` | 400 | 30290 | 11668 | 18622 | 61.48 | 11506.3 | 29870.3 | 13.1ms | 5.0ms | 24.2ms | 128.6ms | no | `200:11668` | `Get "http://127.0.0.1:49283/admin/profiles?limit=20": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-textures` | 1 | 2658 | 2658 | 0 | 0.00 | 2657.3 | 2657.3 | 0.4ms | 0.5ms | 0.6ms | 1.1ms | yes | `200:2658` | `` |
| Admin console | `admin-textures` | 10 | 16921 | 16921 | 0 | 0.00 | 16916.8 | 16916.8 | 0.6ms | 0.5ms | 1.5ms | 2.0ms | yes | `200:16921` | `` |
| Admin console | `admin-textures` | 50 | 20360 | 20360 | 0 | 0.00 | 20324.1 | 20324.1 | 2.3ms | 1.8ms | 5.5ms | 7.5ms | yes | `200:20360` | `` |
| Admin console | `admin-textures` | 100 | 20410 | 20410 | 0 | 0.00 | 20357.7 | 20357.7 | 4.8ms | 4.8ms | 7.3ms | 9.5ms | yes | `200:20410` | `` |
| Admin console | `admin-textures` | 200 | 19507 | 19507 | 0 | 0.00 | 19392.6 | 19392.6 | 10.1ms | 10.0ms | 12.8ms | 17.7ms | yes | `200:19507` | `` |
| Admin console | `admin-textures` | 400 | 30044 | 12850 | 17194 | 57.23 | 12677.3 | 29640.2 | 13.3ms | 4.5ms | 23.9ms | 122.0ms | no | `200:12850` | `Get "http://127.0.0.1:49283/admin/textures?limit=20": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-invites` | 1 | 2679 | 2679 | 0 | 0.00 | 2678.9 | 2678.9 | 0.4ms | 0.5ms | 0.6ms | 1.0ms | yes | `200:2679` | `` |
| Admin console | `admin-invites` | 10 | 19554 | 19554 | 0 | 0.00 | 19544.3 | 19544.3 | 0.5ms | 0.5ms | 1.2ms | 1.7ms | yes | `200:19554` | `` |
| Admin console | `admin-invites` | 50 | 23417 | 23417 | 0 | 0.00 | 23391.8 | 23391.8 | 2.0ms | 1.6ms | 5.1ms | 6.8ms | yes | `200:23417` | `` |
| Admin console | `admin-invites` | 100 | 20708 | 20708 | 0 | 0.00 | 20647.3 | 20647.3 | 4.7ms | 4.5ms | 7.8ms | 9.6ms | yes | `200:20708` | `` |
| Admin console | `admin-invites` | 200 | 21400 | 21400 | 0 | 0.00 | 21264.5 | 21264.5 | 9.3ms | 9.3ms | 12.1ms | 16.4ms | yes | `200:21400` | `` |
| Admin console | `admin-invites` | 400 | 31419 | 15139 | 16280 | 51.82 | 14988.8 | 31107.2 | 12.7ms | 6.0ms | 21.5ms | 79.9ms | no | `200:15139` | `Get "http://127.0.0.1:49283/admin/invites?limit=20": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-settings-site` | 1 | 860 | 860 | 0 | 0.00 | 858.8 | 858.8 | 1.1ms | 1.1ms | 1.8ms | 2.3ms | yes | `200:860` | `` |
| Admin console | `admin-settings-site` | 10 | 7454 | 7454 | 0 | 0.00 | 7443.5 | 7443.5 | 1.3ms | 1.1ms | 2.5ms | 3.0ms | yes | `200:7454` | `` |
| Admin console | `admin-settings-site` | 50 | 8104 | 8104 | 0 | 0.00 | 8063.5 | 8063.5 | 6.1ms | 5.8ms | 9.0ms | 11.3ms | yes | `200:8104` | `` |
| Admin console | `admin-settings-site` | 100 | 7857 | 7857 | 0 | 0.00 | 7804.5 | 7804.5 | 12.7ms | 12.2ms | 16.9ms | 21.2ms | yes | `200:7857` | `` |
| Admin console | `admin-settings-site` | 200 | 8479 | 8479 | 0 | 0.00 | 8377.5 | 8377.5 | 23.7ms | 23.1ms | 29.1ms | 46.9ms | yes | `200:8479` | `` |
| Admin console | `admin-settings-site` | 400 | 25470 | 4811 | 20659 | 81.11 | 4680.7 | 24780.4 | 15.9ms | 3.3ms | 51.6ms | 236.3ms | no | `200:4811` | `Get "http://127.0.0.1:49283/admin/settings/site": dial tcp 127.0.0.1:49283: connectex: No connection could be made because the target machine actively refused it.` |

## Notes

- `Sustainable concurrency` means the highest tested concurrent worker count whose one-second run met the pass condition above.
- `Successful req/s` is the useful per-second throughput at that sustainable concurrency, not merely the number of workers.
- `Tested ceiling? = yes` means the endpoint still passed at the highest configured level; increase `LOADTEST_CONCURRENCY` if you need the actual breaking point.
- This report focuses on realistic frontend page-load endpoints and login; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.
