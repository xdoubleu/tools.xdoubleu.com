FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY --link go.mod go.sum ./
RUN go mod download

COPY --link ./internal/  ./internal/
COPY --link ./apps/ ./apps/
COPY --link ./cmd/publish/ ./cmd/publish/
RUN CGO_ENABLED=0 go build -o=./bin/publish ./cmd/publish

FROM scratch AS server
WORKDIR /app

COPY --from=builder --link /app/bin/publish ./bin/publish

EXPOSE 8000
CMD ["./bin/publish"]
