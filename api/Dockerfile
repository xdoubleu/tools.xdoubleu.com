FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ./bin/api ./cmd/api

FROM gcr.io/distroless/static-debian12 AS server

WORKDIR /app

COPY --from=builder /app/bin/api ./bin/api

EXPOSE 8000

ENTRYPOINT ["/app/bin/api"]
