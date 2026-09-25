# Build context is the repo root: api/go.mod's `replace` needs
# ../sentrytools, so paths are repo-root-relative until WORKDIR /app/api.
FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY sentrytools ./sentrytools
COPY api/go.mod api/go.sum ./api/
WORKDIR /app/api
RUN go mod download

WORKDIR /app
COPY api ./api
WORKDIR /app/api

# RELEASE is read from the env at runtime; no -X stamp.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ./bin/api ./cmd/api

# distroless/static works because PDF→EPUB is pure Go (go-pdfium's
# WebAssembly backend under wazero, no CGO). "nonroot" is uid/gid 65532.
FROM gcr.io/distroless/static-debian12:nonroot AS server

WORKDIR /app

COPY --from=builder /app/api/bin/api ./bin/api

EXPOSE 8000

ENTRYPOINT ["/app/bin/api"]
