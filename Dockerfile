# Two builders and a distroless runtime, so the shipped image carries the panel
# and nothing else: no shell, no package manager, no Node, no Go toolchain.

# --- the UI -------------------------------------------------------------------
FROM node:22-alpine AS ui
WORKDIR /src/web

# Dependencies first, on their own layer: they change far less often than the
# sources do, so a UI edit does not re-download the world.
COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
# Vite is configured to build into internal/web/dist, which is where go:embed
# reaches for it. Embed patterns cannot traverse upwards, hence the location.
RUN npm run build


# --- the binary ---------------------------------------------------------------
FROM golang:1.27-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui /src/internal/web/dist ./internal/web/dist

ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH

# CGO off is not a preference. The SQLite driver is modernc.org/sqlite, which is
# pure Go precisely so this can cross-compile to arm64 and run on a distroless
# base with no libc.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/7dtd-panel ./cmd/7dtd-panel


# --- the data directory -------------------------------------------------------
# Made in a stage that has a shell, because the runtime image does not have one.
# The panel runs as nonroot, and a /data owned by root means SQLite cannot
# create its database — which fails at the first write, not at startup, so it is
# worth getting right here.
FROM alpine:3 AS datadir
RUN mkdir -p /data && chown 65532:65532 /data


# --- what ships ---------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/7dtd-panel /7dtd-panel

# 65532 is nonroot in the distroless images. Docker initialises an empty named
# volume from the image, ownership included, so `docker compose up` works with
# no chown. A bind mount takes the host directory's ownership instead: see the
# README.
COPY --from=datadir --chown=65532:65532 /data /data
VOLUME /data
EXPOSE 8080

# The healthcheck is a subcommand of the same binary, because a distroless image
# has no shell and no curl to call one with. It probes the panel only: a game
# server being down must not make the orchestrator restart the panel.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/7dtd-panel", "healthcheck"]

USER nonroot:nonroot
ENTRYPOINT ["/7dtd-panel"]
