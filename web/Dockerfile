# Cross-compiles the kobo-gateway macOS helper so the web app can serve it
# as a download (see api/internal/kobogateway). Build context is the repo
# root, so api/ is reachable. Runs in parallel with the Node builder.
FROM golang:1.26-alpine AS gateway-builder

ARG RELEASE=dev

WORKDIR /app

COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ .
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
    go build -ldflags="-w -s -X main.Release=${RELEASE}" \
    -o ./bin/kobo-gateway-darwin-arm64 ./cmd/kobo-gateway

FROM node:24-alpine AS builder

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

FROM node:24-alpine AS server

ARG RELEASE=dev
ENV RELEASE=${RELEASE}

WORKDIR /app

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
# Next standalone only serves public/ when it sits next to server.js; the
# directory is assembled here (web/public does not exist in the repo — if it
# ever does, add a `COPY web/public ./public` too).
COPY --from=gateway-builder /app/bin/kobo-gateway-darwin-arm64 \
     ./public/downloads/kobo-gateway-darwin-arm64

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 3000
CMD ["node", "server.js"]
