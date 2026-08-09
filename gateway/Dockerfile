# Builds a standalone image for the gateway binary only — never run
# directly, only used as a `COPY --from=` source by the root Dockerfile,
# which assembles api/gateway/web into the single merged deploy image. See
# root CLAUDE.md's CI section and root Dockerfile.
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# gateway reads RELEASE from the container env at runtime
# (gateway/internal/gateway/config.go), not compile-time.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ./bin/gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot AS server

WORKDIR /app

COPY --from=builder /app/bin/gateway ./bin/gateway

EXPOSE 8000

ENTRYPOINT ["/app/bin/gateway"]
