# Standalone deploy image for the web service (#1038 — deployed
# independently under Kamal, routed by kamal-proxy alongside api; see
# config/deploy.web.yml and root CLAUDE.md's Deploy shape section).
FROM node:24-alpine AS builder

ARG RELEASE=dev
ARG SENTRY_ORG
ARG SENTRY_PROJECT

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

ENV RELEASE=${RELEASE} \
    SENTRY_ORG=${SENTRY_ORG} \
    SENTRY_PROJECT=${SENTRY_PROJECT}
# The auth token is a secret mount (not an ARG) so it never ends up in image
# layers; without it the Sentry plugin skips the source-map upload.
RUN --mount=type=secret,id=sentry_auth_token,env=SENTRY_AUTH_TOKEN \
    npm run build
# .next/standalone excludes .next/static by design (Next expects it served
# separately); fold it in here so the image ships a single, ready-to-COPY
# tree.
RUN cp -r .next/static .next/standalone/.next/static

FROM node:24-alpine AS server

ARG KOBO_GATEWAY_RELEASE=dev
# The release actually baked into the bundled kobo-gateway .dmg/binary — can
# lag behind RELEASE when kobo-gateway's own build was cache-skipped
# (unchanged source). See build-web.yml and lib/env.ts's
# getKoboGatewayRelease().
ENV KOBO_GATEWAY_RELEASE=${KOBO_GATEWAY_RELEASE} \
    PORT=3000 \
    HOSTNAME=0.0.0.0

WORKDIR /app

COPY --from=builder /app/.next/standalone ./

# Next standalone only serves public/ when it sits next to server.js; see
# this file's "Static Downloads" note in web/CLAUDE.md — web/public does not
# exist in the repo otherwise. The kobo-gateway .dmg + raw binary are built
# on macOS by build-kobo-gateway.yml and placed here by build-web.yml before
# `docker build` runs.
COPY public/downloads/kobo-gateway-darwin-arm64 \
     ./public/downloads/kobo-gateway-darwin-arm64
COPY public/downloads/kobo-gateway.dmg \
     ./public/downloads/kobo-gateway.dmg

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 3000

ENTRYPOINT ["node", "server.js"]
