# Railway infrastructure (Cohi)

One file for the whole project: Postgres + MediaMTX + `cohi-api` + `cohi-worker`.

Worker and MediaMTX select their image with `RAILWAY_DOCKERFILE_PATH` (same GitHub repo, different Dockerfiles). JWT / service tokens use `${{secret()}}` so a template deploy generates them.

```bash
npm install railway
railway login
railway link
railway config apply
```

Or GitHub → Actions → **Deploy Railway** (needs repo secret `RAILWAY_TOKEN`).
