# Builds a standalone image for the api binary only — never run directly,
# only used as a `COPY --from=` source by the root Dockerfile, which
# assembles api/gateway/web into the single merged deploy image. See root
# CLAUDE.md's CI section and root Dockerfile.
#
# Build context is the REPO ROOT, not api/ (see build-api.yml's
# `context: .` / `file: ./api/Dockerfile`) — api/go.mod's local `replace
# tools.xdoubleu.com/sentrytools => ../sentrytools` directive needs that
# sibling module inside the build context too, so paths below are
# repo-root-relative until WORKDIR switches to /app/api.
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY sentrytools ./sentrytools
COPY api/go.mod api/go.sum ./api/
WORKDIR /app/api
RUN go mod download

WORKDIR /app
COPY api ./api
WORKDIR /app/api

# api reads RELEASE from the container env at runtime (api/internal/config),
# not compile-time — no -X main.Release=... stamp needed here.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ./bin/api ./cmd/api

# distroless/static: PDF→EPUB conversion no longer shells out to Calibre's
# ebook-convert (Qt/Python) — it's pure Go, using go-pdfium's WebAssembly
# backend (embedded via go:embed, runs under wazero, no CGO, no runtime
# files to ship). CGO_ENABLED=0 above and this static base together drop the
# image from ~700 MB to ~20 MB. "nonroot" already runs as uid/gid 65532.
FROM gcr.io/distroless/static-debian12:nonroot AS server

WORKDIR /app

COPY --from=builder /app/api/bin/api ./bin/api

EXPOSE 8000

ENTRYPOINT ["/app/bin/api"]
