# cohi-api

Control plane for **Cohi / HawkEye** — a self-hostable video surveillance platform.

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/new/template?template=https://github.com/Osawejustice/hawkeye&plugins=postgresql)
[![test](https://github.com/Osawejustice/hawkeye/actions/workflows/test.yml/badge.svg)](https://github.com/Osawejustice/hawkeye/actions/workflows/test.yml)

This service is the source of truth for users, organizations, cameras, and recording metadata. It talks to [MediaMTX](https://github.com/bluenviron/mediamtx) over its Control API to project camera records onto live paths (RTSP ingest, HLS / WebRTC egress).

## What this version includes

- Register / login / refresh / logout with JWT access tokens + rotatable refresh tokens
- Current-user profile (`GET` / `PATCH /api/v1/users/me`)
- Camera CRUD with organization ownership
- MediaMTX path sync (add / replace / delete) behind a clean client interface
- Recording index (`recording_segments`) + authenticated MP4 playback proxied from MediaMTX
- Events (camera online/offline, new segments, camera created/deleted)
- `cohi-worker` — polls MediaMTX playback + path status, upserts the index
- PostgreSQL schema via golang-migrate (soft deletes, multi-tenancy-ready `organization_id`)
- Health / readiness probes
- Production multi-stage Dockerfile (api + worker targets) and local docker-compose (Postgres + MediaMTX + demo source + API + worker)
- Railway one-click / GitHub auto-deploy (`railway.toml`, production secrets, Postgres)

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
Worker health: `http://localhost:8081/health`  
Postgres: `localhost:5432` (`cohi` / `cohi` / `cohi`)  
MediaMTX Control API: `http://localhost:9997`  
Demo RTSP (test pattern): `rtsp://127.0.0.1:8554/demo`

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
| `MEDIAMTX_PLAYBACK_URL` | Internal Playback API used to stream recordings |
| `INTERNAL_SERVICE_TOKEN` | Shared secret for `/internal/v1` (worker) |
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
| `GET` | `/api/v1/cameras/:id/recordings` | Segments for one camera |

### Recordings

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/v1/recordings` | List (`camera_id`, `from`, `to`, `page`, `per_page`) |
| `GET` | `/api/v1/recordings/:id` | Metadata |
| `GET` | `/api/v1/recordings/:id/video` | Authenticated MP4 stream (proxied from MediaMTX playback) |

### Events

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/v1/events` | List (`camera_id`, `type`, `page`, `per_page`) |

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
    "recording_enabled": true
  }'
```

The dashboard live view supports HLS and WebRTC (WHEP). Toggle them on the camera page.

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
cmd/api/                    control-plane entrypoint
cmd/worker/                 recording-worker entrypoint
internal/app/               composition root
internal/worker/            poll loop + internal API client
internal/storage/           playback backend (MediaMTX now, R2 later)
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
deploy/railway.variables.env  production env paste for Railway
railway.toml / railway.json Railway config-as-code
.github/workflows/          CI (go test / vet)
```

## Auth design

- Access token: HS256 JWT, 15 minutes, claims `uid`, `oid`, `email`, `role`
- Refresh token: 256-bit opaque value, stored as HMAC-SHA256, 7 days
- Rotation on every refresh; reuse of a revoked token invalidates the whole family
- Passwords: bcrypt cost 12

## Deploy on Railway

This repo is a Railway template in the same shape as a one-click Docker template: a production Dockerfile, `railway.toml` / `railway.json`, and PostgreSQL as the only datastore.

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/new/template?template=https://github.com/Osawejustice/hawkeye&plugins=postgresql)

PostgreSQL only. MySQL is not supported.

### One-click

1. Click **Deploy on Railway**.
2. Railway provisions PostgreSQL and builds this Dockerfile (final stage = `api`).
3. Open the `cohi-api` service → **Variables** and paste [deploy/railway.variables.env](deploy/railway.variables.env). Fill the three secrets:

```bash
openssl rand -base64 48   # JWT_ACCESS_SECRET
openssl rand -base64 48   # JWT_REFRESH_SECRET
openssl rand -base64 32   # INTERNAL_SERVICE_TOKEN
```

4. Set `DATABASE_URL` to the reference `${{Postgres.DATABASE_URL}}` (already in the paste file).
5. Generate a public domain on the service. Health check is `GET /health`.
6. After ~1 minute the API is up. Migrations run on boot (`AUTO_MIGRATE=true`).

```bash
curl https://<your-service>.up.railway.app/health
curl -sS -X POST https://<your-service>.up.railway.app/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"ops@example.com","password":"correct-horse","name":"Ops"}'
```

### Deploy from this GitHub repo (recommended for Cohi)

1. Push `main` to GitHub (this repository).
2. Railway dashboard → **New Project** → **GitHub Repo** → `hawkeye`.
3. Railway detects the Dockerfile and `railway.toml`.
4. **+ New** → **Database** → **PostgreSQL**.
5. On the API service, add the variables from [deploy/railway.variables.env](deploy/railway.variables.env).
6. Enable a public domain. Subsequent pushes to `main` auto-deploy.

MediaMTX is **not** part of this Railway service. Live ingest / playback stay on the local compose stack (or a later media-server service). In production, localhost MediaMTX URLs are disabled automatically so camera CRUD still works (`mtx_sync_status: skipped`).

If you already have a Railway Postgres (or any Postgres 16), skip step 4 and set `DATABASE_URL` to that instance. Do not point this API at MySQL.

## recording-worker

Same module, separate binary (`cmd/worker`). It does **not** own the schema.

Every poll interval it:

1. Lists cameras from `GET /internal/v1/cameras`
2. Reads MediaMTX path status and POSTs heartbeats (`is_online` / `last_seen_at` + events)
3. Re-creates recording-enabled paths that disappeared after a MediaMTX restart
4. Lists playback segments per recording-enabled camera and upserts `recording_segments`

The API also re-projects every camera onto MediaMTX at boot (paths added via the Control API are in-memory).

```bash
docker compose up -d worker
# or
make run-worker
```

Internal routes require `X-Service-Token` / `Authorization: Bearer` matching `INTERNAL_SERVICE_TOKEN`.

## Demo camera

Compose publishes a test pattern to `rtsp://127.0.0.1:8554/demo`. Register, then create a camera with that RTSP URL and recording enabled. After ~1 minute the worker indexes a clip; play it from the web app or:

```bash
curl -L -H "Authorization: Bearer $ACCESS" \
  http://localhost:8080/api/v1/recordings
```

## Next services (not this repo)

- Motion detection inside recording-worker
- Object storage (Cloudflare R2) behind `internal/storage` (interface already exists)
- ONVIF discovery (manual RTSP is first)
- Event fan-out (SSE / WebSocket / notifications)
