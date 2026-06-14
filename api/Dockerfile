FROM golang:1.26-alpine AS builder

ARG RELEASE=dev

WORKDIR /app

COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Release=${RELEASE}" -o ./bin/api ./cmd/api

# debian:12-slim is required (not distroless) because Calibre's ebook-convert
# subprocess needs Qt/Python shared libraries. Image is significantly larger as
# a result; the Calibre layer is cached via GitHub Actions layer caching.
FROM debian:12-slim AS server

RUN apt-get update \
    && apt-get install -y --no-install-recommends calibre \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd --gid 65532 nonroot && useradd --uid 65532 --gid 65532 --no-create-home nonroot

WORKDIR /app

COPY --from=builder /app/bin/api ./bin/api

USER nonroot:nonroot

EXPOSE 8000

ENTRYPOINT ["/app/bin/api"]
