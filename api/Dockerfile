# Build context is the repo root because the web build needs web/; paths are
# repo-root-relative until WORKDIR /app/api.
FROM node:26-alpine@sha256:c6ad9e9c1d80f6eb607c26b4b0a132ff7a2344fad1c78ea3baa2a7d4c8de6d94 AS web-builder

ARG RELEASE=dev
ARG SENTRY_ORG
ARG SENTRY_PROJECT

WORKDIR /app/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

# build-api.yml stages the kobo-gateway downloads into web/public/downloads
# before `docker build`; Vite copies public/ into dist/, so the api serves them
# at /downloads/. The .sig is present only on signed (main) builds.
COPY web ./

ENV RELEASE=${RELEASE} \
    SENTRY_ORG=${SENTRY_ORG} \
    SENTRY_PROJECT=${SENTRY_PROJECT}
# The auth token is a secret mount (not an ARG) so it never ends up in image
# layers; without it the Sentry plugin skips the source-map upload.
RUN --mount=type=secret,id=sentry_auth_token,env=SENTRY_AUTH_TOKEN \
    npm run build

FROM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

WORKDIR /app

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
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS server

# May lag RELEASE when kobo-gateway's build cache-hit (see web/lib/env.ts).
ARG KOBO_GATEWAY_RELEASE=dev
ENV KOBO_GATEWAY_RELEASE=${KOBO_GATEWAY_RELEASE} \
    WEB_DIR=/app/web

WORKDIR /app

COPY --from=builder /app/api/bin/api ./bin/api
COPY --from=web-builder /app/web/dist ./web

EXPOSE 8000

ENTRYPOINT ["/app/bin/api"]
