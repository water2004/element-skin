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
