# cohi-api

Control plane for **Cohi / HawkEye** — a self-hostable video surveillance platform.

This service is the source of truth for users, organizations, cameras, and recording metadata. It talks to [MediaMTX](https://github.com/bluenviron/mediamtx) over its Control API to project camera records onto live paths (RTSP ingest, HLS / WebRTC egress).

## What this version includes

- Register / login / refresh / logout with JWT access tokens + rotatable refresh tokens
- Current-user profile (`GET` / `PATCH /api/v1/users/me`)
- Camera CRUD with organization ownership
- MediaMTX path sync (add / replace / delete) behind a clean client interface
- PostgreSQL schema via golang-migrate (soft deletes, multi-tenancy-ready `organization_id`)
- Health / readiness probes
- Production multi-stage Dockerfile and local docker-compose (Postgres + MediaMTX + API)

## Requirements

- Go 1.22+
- Docker (for local Postgres / MediaMTX), or a reachable PostgreSQL 16 instance
- `make` (optional)

## Quick start (Docker)

```bash
cp .env.example .env
# edit JWT secrets in .env before any real deployment

docker compose up --build
```

API: `http://localhost:8080`  
Postgres: `localhost:5432` (`cohi` / `cohi` / `cohi`)  
MediaMTX Control API: `http://localhost:9997`

Migrations run automatically on API startup when `AUTO_MIGRATE=true`.

## Quick start (local Go + Docker Postgres)

```bash
cp .env.example .env
docker compose up -d postgres mediamtx
make run
```

Hot reload (optional):

```bash
go install github.com/air-verse/air@latest
make air
```

## Environment

See [`.env.example`](.env.example). Important variables:

| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | PostgreSQL DSN |
| `AUTO_MIGRATE` | Run golang-migrate on boot (default `true`) |
| `JWT_ACCESS_SECRET` / `JWT_REFRESH_SECRET` | ≥32 chars in production |
| `JWT_ACCESS_TTL` / `JWT_REFRESH_TTL` | e.g. `15m` / `168h` |
| `MEDIAMTX_API_URL` | Internal Control API (`http://mediamtx:9997` in compose) |
| `MEDIAMTX_RTSP_URL` / `HLS` / `WEBRTC` | **Public** URLs returned to clients |
| `PORT` | Overrides `HTTP_PORT` (Railway-compatible) |

## API

All JSON responses are wrapped:

```json
{ "success": true, "data": { } }
```

Errors:

```json
{ "success": false, "error": { "code": "invalid_credentials", "message": "invalid email or password" } }
```

Protected routes require `Authorization: Bearer <access_token>`.

### Health

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | no | Liveness |
| `GET` | `/ready` | no | Readiness (Postgres ping) |

### Auth

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/auth/register` | no | Create org + admin user, return tokens |
| `POST` | `/api/v1/auth/login` | no | Login |
| `POST` | `/api/v1/auth/refresh` | no | Rotate refresh token, issue new pair |
| `POST` | `/api/v1/auth/logout` | no | Revoke a refresh token |
| `POST` | `/api/v1/auth/logout-all` | yes | Revoke every session for the current user |

### Users

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/v1/users/me` | Current user |
| `PATCH` | `/api/v1/users/me` | Update name / password |

### Cameras

Cameras belong to the caller's organization. Update/delete: owner **or** org admin.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/v1/cameras` | List (`q`, `enabled`, `page`, `per_page`) |
| `POST` | `/api/v1/cameras` | Create + sync MediaMTX path |
| `GET` | `/api/v1/cameras/:id` | Get |
| `PATCH` | `/api/v1/cameras/:id` | Update (re-syncs path if stream fields change) |
| `DELETE` | `/api/v1/cameras/:id` | Soft-delete + remove MediaMTX path |
| `POST` | `/api/v1/cameras/:id/sync` | Retry MediaMTX projection |
| `GET` | `/api/v1/cameras/:id/status` | Live MediaMTX path status |

## End-to-end: Register → Login → Create Camera

```bash
# 1. Register
curl -sS -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "ops@example.com",
    "password": "correct-horse",
    "name": "Ops"
  }'
```

Save `data.tokens.access_token` as `ACCESS`.

```bash
# 2. Login (if you already have an account)
curl -sS -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"ops@example.com","password":"correct-horse"}'
```

```bash
# 3. Create a camera
curl -sS -X POST http://localhost:8080/api/v1/cameras \
  -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Front Gate",
    "location": "Entrance",
    "rtsp_url": "rtsp://192.168.1.20:554/Streaming/Channels/101",
    "rtsp_username": "admin",
    "rtsp_password": "secret",
    "enabled": true,
    "recording_enabled": false
  }'
```

The response includes playback URLs under `data.stream`:

```json
{
  "path": "cam-<uuid>",
  "rtsp": "rtsp://localhost:8554/cam-<uuid>",
  "hls": "http://localhost:8888/cam-<uuid>/index.m3u8",
  "webrtc": "http://localhost:8889/cam-<uuid>",
  "whep": "http://localhost:8889/cam-<uuid>/whep"
}
```

If MediaMTX is down the camera is still created (`mtx_sync_status: "error"`). Retry with `POST /api/v1/cameras/:id/sync`.

Refresh:

```bash
curl -sS -X POST http://localhost:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"'"$REFRESH"'"}'
```

Logout:

```bash
curl -sS -X POST http://localhost:8080/api/v1/auth/logout \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"'"$REFRESH"'"}'
```

## Project layout

```
cmd/api/                    entrypoint
internal/app/               composition root
internal/auth/              password hashing + JWT
internal/config/            environment loading
internal/database/          GORM + golang-migrate
internal/http/              router, server, handlers, DTOs
internal/http/middleware/   auth, CORS, request id, rate limit
internal/media/mediamtx/    MediaMTX Control API client
internal/models/            GORM models
internal/repository/        persistence
internal/service/           business logic
migrations/                 SQL (golang-migrate)
deployments/mediamtx.yml    local MediaMTX config
```

## Auth design

- Access token: HS256 JWT, 15 minutes, claims `uid`, `oid`, `email`, `role`
- Refresh token: 256-bit opaque value, stored as HMAC-SHA256, 7 days
- Rotation on every refresh; reuse of a revoked token invalidates the whole family
- Passwords: bcrypt cost 12

## Railway

1. Provision PostgreSQL and set `DATABASE_URL`
2. Set `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, `APP_ENV=production`
3. `AUTO_MIGRATE=true` (or run migrations in a release command)
4. `PORT` is honored automatically
5. Point `MEDIAMTX_API_URL` at the media-server service when it is deployed

## Next services (not this repo)

- `recording-worker` — motion / segment management, writes `recording_segments`
- Object storage (Cloudflare R2) behind a storage interface
- ONVIF discovery (manual RTSP is first)
