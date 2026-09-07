# Railway infrastructure (Cohi)

This file describes the **whole** project: Postgres + MediaMTX + `cohi-api` + `cohi-worker`.

`railway.toml` / `railway.json` only configure a single service. They cannot create the API next to Postgres. Use this graph instead, or add the services in the dashboard (see the repo README).

## Apply (needs Railway CLI login)

```bash
npm install railway
railway login
railway link          # choose project upbeat-success / production
railway config plan
railway config apply
```

Then set Dockerfile paths (same GitHub repo, different images):

| Service | Dockerfile path |
| --- | --- |
| `cohi-api` | `Dockerfile` (default) |
| `cohi-worker` | `Dockerfile.worker` |
| `mediamtx` | `Dockerfile.mediamtx` |

Paste JWT secrets on `cohi-api` and `cohi-worker` from `deploy/railway.variables.env`. Generate a public domain on `cohi-api`.
