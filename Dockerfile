# ---- frontend ----
FROM node:22-alpine AS frontend
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---- backend ----
FROM golang:1.25-bookworm AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /src/build ./web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /couchverse ./cmd/couchverse

# ---- runtime ----
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates tzdata wget \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -r -u 1000 -m couchverse \
    && mkdir -p /data && chown couchverse /data
COPY --from=backend /couchverse /usr/local/bin/couchverse
USER couchverse
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s \
    CMD wget -qO- http://localhost:8080/healthz || exit 1
ENTRYPOINT ["couchverse"]
