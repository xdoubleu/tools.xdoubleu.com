# Builds a standalone image for the api binary only — never run directly,
# only used as a `COPY --from=` source by the root Dockerfile, which
# assembles api/gateway/web into the single merged deploy image. See root
# CLAUDE.md's CI section and root Dockerfile.
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

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

COPY --from=builder /app/bin/api ./bin/api

EXPOSE 8000

ENTRYPOINT ["/app/bin/api"]
