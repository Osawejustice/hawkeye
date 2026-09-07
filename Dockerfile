# syntax=docker/dockerfile:1
#
# Multi-stage image. The final stage is `api` so Railway / `docker build`
# produce the control plane by default. Build the worker with:
#   docker build --target worker -t cohi-worker .

FROM golang:1.22-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=0.2.0
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w -X github.com/cohi-hq/cohi-api/internal/app.Version=${VERSION}" \
    -o /out/cohi-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" \
    -o /out/cohi-worker ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot AS worker
WORKDIR /app
COPY --from=build /out/cohi-worker /app/cohi-worker
ENV HTTP_HOST=0.0.0.0
ENV HTTP_PORT=8081
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/app/cohi-worker"]

FROM gcr.io/distroless/static-debian12:nonroot AS api
WORKDIR /app
COPY --from=build /out/cohi-api /app/cohi-api
COPY migrations /app/migrations
ENV MIGRATIONS_PATH=/app/migrations
ENV HTTP_HOST=0.0.0.0
ENV HTTP_PORT=8080
ENV AUTO_MIGRATE=true
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/cohi-api"]
