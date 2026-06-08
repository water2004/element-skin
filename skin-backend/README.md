# Element Skin Go Backend

This is the Go implementation of the Element Skin backend. It keeps the public
HTTP/Yggdrasil surface required by the current frontend while keeping the
runtime small, explicit, and centered on the Go domain modules.

## Layout

- `cmd/element-skin`: process entrypoint and graceful HTTP shutdown.
- `internal/app`: application assembly, database init, router wiring, and
  refresh-token cleanup lifecycle.
- `internal/config`: YAML loading and nested config normalization.
- `internal/database`: PostgreSQL schema and store methods. Shared mutations
  that must be atomic live here.
- `internal/httpapi`: HTTP route adapters. Business rules should stay in
  `internal/service` or `internal/database` unless they are purely request/response
  concerns.
- `internal/service`: site, Yggdrasil, fallback, Microsoft/import, settings, and
  texture-storage domain logic.
- `internal/util`: small security, pagination, JWT, URL, and response helpers.
- `internal/integration`: end-to-end backend tests against a real PostgreSQL
  test database.

## Local Checks

Use repo-local caches so test/build output stays inside the workspace:

```powershell
$env:GOCACHE='D:\element-skin\.gocache'
$env:GOMODCACHE='D:\element-skin\.gomodcache'
go test ./...
go vet ./...
go build ./cmd/element-skin
```

## Load Testing

`cmd/loadtest` runs an opt-in concurrency ladder against a running backend. It is
kept outside the default test suite because it measures the current machine,
PostgreSQL, and network path rather than a stable unit-test invariant.

To measure a manually started backend, start the backend first, then run:

```powershell
go run ./cmd/loadtest -target http://127.0.0.1:8000 -path /public/settings -concurrency 1,5,10,25,50,100 -duration 10s
```

For authenticated frontend endpoints, let the tool log in once and reuse the
returned cookies:

```powershell
go run ./cmd/loadtest -target http://127.0.0.1:8000 -path /me -login-email user@example.com -login-password Password123 -concurrency 1,5,10,25,50 -duration 10s
```

The final "Suggested capacity" is the highest tested concurrency whose failure
rate is below `-fail-threshold` and, when set, whose p95 latency is below
`-max-p95`.

For a cleaner real-backend test that does not touch the normal configured
database, run the opt-in test harness. It creates an isolated PostgreSQL test
database, seeds users/profiles/textures, starts an in-process HTTP server, and
then runs the same concurrency ladder against real routes:

```powershell
$env:LOADTEST_ENABLE='1'
$env:LOADTEST_CONCURRENCY='1,10,50,100,200'
$env:LOADTEST_DURATION='5s'
go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v
```

By default the harness writes `../reports/concurrency-load-test.md`; override it
with `LOADTEST_REPORT` when you want a different report path.

The harness uses `TEST_DATABASE_DSN`/`ADMIN_DATABASE_DSN` when set, otherwise it
follows the same local PostgreSQL defaults as the integration tests.

The integration tests create isolated PostgreSQL databases through
`internal/testutil`; they exercise the HTTP router, services, stores, token
rotation, import flows, fallback dispatch, pagination, and important failure
paths used by the current frontend and Yggdrasil clients.

## Design Notes

- Authentication checks re-read the user from the database on each protected
  request, so deleted users and demoted admins lose access immediately even if
  their JWT has not expired.
- Site refresh tokens are one-shot and consumed atomically in PostgreSQL.
- Registration creates the user and initial profile in one transaction, including
  invite consumption, so profile or invite failures do not leave orphan users.
- Texture uploads share a single multipart reader with a hard byte limit; oversized
  uploads are rejected instead of being silently truncated.
- Generic internal errors return a stable public message. User-facing business
  errors use `util.HTTPError`.
- Outbound texture downloads validate URLs and enforce a hard response-size cap.

## Docker

`../docker-compose.yml` builds this backend through `skin-backend/Dockerfile`.
The Dockerfile builds the frontend first, then compiles a static Go binary and
serves both from the runtime image.
