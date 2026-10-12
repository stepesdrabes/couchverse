# ---- core (wasm) ----
# The wasm is the same on every platform, so it is built once on the build host rather than
# under emulation for each image platform.
FROM --platform=$BUILDPLATFORM rust:1.99-slim-bookworm AS core
ARG BUILDARCH
ARG BINARYEN_VERSION=133
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && rustup target add wasm32-unknown-unknown \
    && arch=$([ "$BUILDARCH" = arm64 ] && echo aarch64 || echo x86_64) \
    && curl -sSL "https://github.com/WebAssembly/binaryen/releases/download/version_${BINARYEN_VERSION}/binaryen-version_${BINARYEN_VERSION}-${arch}-linux.tar.gz" \
        | tar xz -C /opt \
    && ln -s /opt/binaryen-version_${BINARYEN_VERSION}/bin/wasm-opt /usr/local/bin/wasm-opt
WORKDIR /repo
COPY contract/design /repo/contract/design
COPY core/ /repo/core/
# CI makes a missing wasm-opt an error rather than a warning
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/repo/core/target \
    cd core && CI=true cargo xtask wasm

# ---- web ----
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /repo/clients/web
COPY clients/web/package.json clients/web/package-lock.json ./
RUN npm ci
COPY contract/i18n /repo/contract/i18n
# the admin Broadcast preview compiles the broadcast package from source
COPY clients/broadcast/src /repo/clients/broadcast/src
COPY clients/web/ ./
COPY --from=core /repo/clients/web/src/lib/core/pkg ./src/lib/core/pkg
RUN npm run build

# ---- broadcast ----
# Couch TV's render page, which the binary embeds for the render sidecar
FROM --platform=$BUILDPLATFORM node:22-alpine AS broadcast
WORKDIR /repo/clients/broadcast
COPY clients/broadcast/package.json clients/broadcast/package-lock.json ./
RUN npm ci
COPY clients/broadcast/ ./
RUN npm run build

# ---- backend ----
# Go cross-compiles, so this too runs on the build host instead of under emulation.
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS gosrc
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=broadcast /repo/clients/broadcast/dist ./broadcast/dist

FROM gosrc AS backend
ARG TARGETOS
ARG TARGETARCH
COPY --from=web /repo/clients/web/build ./web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X couchverse/internal/version.Version=${VERSION}" -o /couchverse ./cmd/couchverse

# the sidecar's build of the same binary leaves the SPA out, so it skips the core and web
FROM gosrc AS render-backend
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN mkdir -p web/dist && touch web/dist/.gitkeep \
    && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X couchverse/internal/version.Version=${VERSION}" -o /couchverse ./cmd/couchverse

# ---- render sidecar ----
# `docker build --target render`: `couchverse render` with Chromium, ffmpeg and Mesa's GL and
# EGL drivers for the GPU (docs/couch-tv-plan.md 6.7); the bundle brings its own fonts. The
# app image below stays as slim as it was.
FROM debian:trixie-slim AS render
RUN apt-get update \
    && apt-get install -y --no-install-recommends chromium ffmpeg ca-certificates tzdata \
        libgl1-mesa-dri libegl1 libegl-mesa0 libgles2 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -r -u 1000 -m couchverse \
    && mkdir -p /data && chown couchverse /data
COPY --from=render-backend /couchverse /usr/local/bin/couchverse
ENV CHROMIUM_PATH=/usr/bin/chromium
USER couchverse
ENTRYPOINT ["couchverse", "render"]

# ---- runtime ----
FROM debian:trixie-slim
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
