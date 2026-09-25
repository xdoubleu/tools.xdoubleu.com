# Standalone deploy image for the web service (config/deploy.web.yml).
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
# .next/standalone excludes .next/static; fold it in.
RUN cp -r .next/static .next/standalone/.next/static

FROM node:24-alpine AS server

ARG KOBO_GATEWAY_RELEASE=dev
# May lag RELEASE when kobo-gateway's build cache-hit (see lib/env.ts).
ENV KOBO_GATEWAY_RELEASE=${KOBO_GATEWAY_RELEASE} \
    PORT=3000 \
    HOSTNAME=0.0.0.0

WORKDIR /app

COPY --from=builder /app/.next/standalone ./

# Standalone only serves public/ next to server.js. build-web.yml stages the
# kobo-gateway downloads here before `docker build`.
COPY public/downloads/kobo-gateway-darwin-arm64 \
     ./public/downloads/kobo-gateway-darwin-arm64
COPY public/downloads/kobo-gateway.dmg \
     ./public/downloads/kobo-gateway.dmg

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 3000

ENTRYPOINT ["node", "server.js"]
