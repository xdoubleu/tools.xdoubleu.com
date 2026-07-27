FROM golang:1.26-alpine AS builder

ARG RELEASE=dev

WORKDIR /app

COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Release=${RELEASE}" -o ./bin/api ./cmd/api

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
