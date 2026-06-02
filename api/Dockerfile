FROM golang:1.26-alpine AS builder

ARG RELEASE=dev

WORKDIR /app

COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Release=${RELEASE}" -o ./bin/api ./cmd/api

FROM gcr.io/distroless/static-debian12 AS server

WORKDIR /app

COPY --from=builder /app/bin/api ./bin/api

USER nonroot:nonroot

EXPOSE 8000

ENTRYPOINT ["/app/bin/api"]
