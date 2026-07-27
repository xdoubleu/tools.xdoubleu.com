FROM golang:1.26-alpine AS go-builder

ARG RELEASE=dev

WORKDIR /app

COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Release=${RELEASE}" -o ./bin/api ./cmd/api

FROM node:24-alpine AS web-builder

ARG RELEASE=dev
ARG SENTRY_ORG
ARG SENTRY_PROJECT

WORKDIR /app

COPY web/package.json web/package-lock.json ./

RUN npm ci

COPY web/app app
COPY web/components components
COPY web/hooks hooks
COPY web/lib lib
COPY web/next.config.ts web/tsconfig.json web/postcss.config.js \
     web/instrumentation-client.ts \
     web/sentry.edge.config.ts web/sentry.server.config.ts \
     web/next-env.d.ts ./

ENV RELEASE=${RELEASE} \
    SENTRY_ORG=${SENTRY_ORG} \
    SENTRY_PROJECT=${SENTRY_PROJECT}
# The auth token is a secret mount (not an ARG) so it never ends up in image
# layers; without it the Sentry plugin skips the source-map upload.
RUN --mount=type=secret,id=sentry_auth_token,env=SENTRY_AUTH_TOKEN \
    npm run build

# Merged single-component image (issue #558): one process, the Go binary,
# runs as PID 1 and spawns `node server.js` as a supervised child
# (cmd/api/web_process.go), front-dooring it with a reverse proxy
# (cmd/api/frontend_proxy.go) that replicates the two DO ingress rules the
# separate api/web components used to get for free. The base has to be
# node:24-alpine rather than distroless because the image now carries the
# Node runtime regardless — this is the exact base web/Dockerfile validated
# on before the merge. #588's distroless win survives as a *memory* win (a
# static, CGO-free Go binary with no Qt/Python peak) rather than an
# image-size one — see api/CLAUDE.md.
FROM node:24-alpine AS server

# The Go binary calls out to Supabase, R2, GitHub, Sentry, arXiv, Hardcover,
# UniCat and Resend over HTTPS; distroless/static supplied the CA bundle
# before, alpine doesn't by default.
RUN apk add --no-cache ca-certificates

WORKDIR /app

ENV WEB_ENABLED=true \
    WEB_SERVER_JS=/app/web/server.js

COPY --from=go-builder /app/bin/api ./bin/api

COPY --from=web-builder /app/.next/standalone ./web/
COPY --from=web-builder /app/.next/static ./web/.next/static
# Next standalone only serves public/ when it sits next to server.js; see
# web/CLAUDE.md's "Static Downloads" note — web/public does not exist in the
# repo otherwise. kobo-gateway.dmg + the raw binary are built on macOS by
# build-gateway.yml and placed here by docker.yml before `docker build` runs.
COPY web/public/downloads/kobo-gateway-darwin-arm64 \
     ./web/public/downloads/kobo-gateway-darwin-arm64
COPY web/public/downloads/kobo-gateway.dmg \
     ./web/public/downloads/kobo-gateway.dmg

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 8000

ENTRYPOINT ["/app/bin/api"]
