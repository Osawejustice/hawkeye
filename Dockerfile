# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=0.1.0
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X github.com/cohi-hq/cohi-api/internal/app.Version=${VERSION}" \
    -o /out/cohi-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=build /out/cohi-api /app/cohi-api
COPY migrations /app/migrations

ENV MIGRATIONS_PATH=/app/migrations
ENV HTTP_HOST=0.0.0.0
ENV HTTP_PORT=8080

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/cohi-api"]
