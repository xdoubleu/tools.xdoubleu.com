# Builds a standalone image carrying web's `.next/standalone` + `.next/static`
# output only — never run directly, only used as a `COPY --from=` source by
# the root Dockerfile, which assembles api/gateway/web into the single
# merged deploy image. See root CLAUDE.md's CI section and root Dockerfile.
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

# Never executed — just a file carrier for `COPY --from=web` in the root
# Dockerfile, so `scratch` is sufficient.
FROM scratch AS server

COPY --from=builder /app/.next/standalone /
