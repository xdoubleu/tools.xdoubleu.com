# Builds a standalone image for the gateway binary only — never run
# directly, only used as a `COPY --from=` source by the root Dockerfile,
# which assembles api/gateway/web into the single merged deploy image. See
# root CLAUDE.md's CI section and root Dockerfile.
#
# Build context is the REPO ROOT, not gateway/ (see build-gateway.yml's
# `context: .` / `file: ./gateway/Dockerfile`) — gateway/go.mod's local
# `replace tools.xdoubleu.com/sentrytools => ../sentrytools` directive needs
# that sibling module inside the build context too, so paths below are
# repo-root-relative until WORKDIR switches to /app/gateway.
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY sentrytools ./sentrytools
COPY gateway/go.mod gateway/go.sum ./gateway/
WORKDIR /app/gateway
RUN go mod download

WORKDIR /app
COPY gateway ./gateway
WORKDIR /app/gateway

# gateway reads RELEASE from the container env at runtime
# (gateway/internal/gateway/config.go), not compile-time.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ./bin/gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot AS server

WORKDIR /app

COPY --from=builder /app/gateway/bin/gateway ./bin/gateway

EXPOSE 8000

ENTRYPOINT ["/app/bin/gateway"]
