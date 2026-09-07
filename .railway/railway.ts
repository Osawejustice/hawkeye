import {
  defineRailway,
  github,
  postgres,
  preserve,
  project,
  service,
  volume,
} from "railway/iac";

const REPO = "Osawejustice/hawkeye";

export default defineRailway(() => {
  const db = postgres("Postgres");

  const recordings = volume("recordings-data", {
    sizeMB: 5000,
  });

  const mediamtx = service("mediamtx", {
    source: github(REPO, { branch: "main" }),
    volumeMounts: {
      "/recordings": recordings,
    },
    env: {
      MTX_LOGLEVEL: "info",
    },
  });

  const api = service("cohi-api", {
    source: github(REPO, { branch: "main" }),
    healthcheck: "/health",
    healthcheckTimeout: 120,
    env: {
      APP_ENV: "production",
      AUTO_MIGRATE: "true",
      LOG_LEVEL: "info",
      LOG_FORMAT: "json",
      CORS_ALLOWED_ORIGINS: "*",
      MEDIAMTX_ENABLED: "true",
      DATABASE_URL: db.env.DATABASE_URL,
      MEDIAMTX_API_URL: "http://mediamtx.railway.internal:9997",
      MEDIAMTX_PLAYBACK_URL: "http://mediamtx.railway.internal:9996",
      MEDIAMTX_RTSP_URL: "rtsp://mediamtx.railway.internal:8554",
      MEDIAMTX_HLS_URL: "http://mediamtx.railway.internal:8888",
      MEDIAMTX_WEBRTC_URL: "http://mediamtx.railway.internal:8889",
      JWT_ACCESS_SECRET: preserve(),
      JWT_REFRESH_SECRET: preserve(),
      INTERNAL_SERVICE_TOKEN: preserve(),
    },
  });

  const worker = service("cohi-worker", {
    source: github(REPO, { branch: "main" }),
    healthcheck: "/health",
    healthcheckTimeout: 60,
    env: {
      APP_ENV: "production",
      COHI_API_URL: "http://cohi-api.railway.internal:${{cohi-api.PORT}}",
      INTERNAL_SERVICE_TOKEN: preserve(),
      WORKER_POLL_INTERVAL: "15s",
      MEDIAMTX_ENABLED: "true",
      MEDIAMTX_API_URL: "http://mediamtx.railway.internal:9997",
      MEDIAMTX_PLAYBACK_URL: "http://mediamtx.railway.internal:9996",
    },
  });

  return project("upbeat-success", {
    resources: [db, recordings, mediamtx, api, worker],
  });
});
