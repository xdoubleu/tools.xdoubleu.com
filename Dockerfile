# Assembly-only: api/gateway/web/kobo-gateway are each built and cached by
# their own CI job (build-api.yml/build-gateway.yml/build-web.yml/
# build-kobo-gateway.yml — a component whose source didn't change reuses
# last build's binary instead of recompiling here) and downloaded by
# docker.yml into the paths this COPYs from before `docker build` runs. No
# compilation happens in this Dockerfile at all — see root CLAUDE.md's CI
# section.
#
# Merged single-component image (issue #558; split into 3 processes in
# #904): the `gateway` binary runs as PID 1 and spawns both `api` and `node
# server.js` as supervised children (gateway/internal/gateway),
# reverse-proxying between them to replicate the two DO ingress rules the
# separate api/web components used to get for free — see gateway/CLAUDE.md.
# The base has to be node:24-alpine rather than distroless because the
# image now carries the Node runtime regardless — this is the exact base
# web/Dockerfile validated on before the merge. #588's distroless win
# survives as a *memory* win (static, CGO-free Go binaries with no
# Qt/Python peak) rather than an image-size one — see api/CLAUDE.md.
FROM node:24-alpine AS server

ARG RELEASE=dev
# The release actually baked into the bundled kobo-gateway .dmg/binary —
# can lag behind RELEASE when kobo-gateway's own build was skipped
# (unchanged source). See build-kobo-gateway.yml and
# gateway/internal/gateway/config.go.
ARG KOBO_GATEWAY_RELEASE=dev

# The Go binaries call out to Supabase, R2, GitHub, Sentry, arXiv,
# Hardcover, UniCat and Resend over HTTPS; distroless/static supplied the CA
# bundle before, alpine doesn't by default.
RUN apk add --no-cache ca-certificates

WORKDIR /app

ENV WEB_SERVER_JS=/app/web/server.js \
    API_BIN_PATH=/app/bin/api \
    RELEASE=${RELEASE} \
    KOBO_GATEWAY_RELEASE=${KOBO_GATEWAY_RELEASE}

COPY bin/api ./bin/api
COPY bin/gateway ./bin/gateway
RUN chmod +x ./bin/api ./bin/gateway

# Fully-assembled .next/standalone + .next/static, built and combined by
# build-web.yml — see that workflow for why static is folded in here rather
# than copied separately (Next's standalone output excludes it by design).
COPY web-standalone/ ./web/

# Next standalone only serves public/ when it sits next to server.js; see
# web/CLAUDE.md's "Static Downloads" note — web/public does not exist in the
# repo otherwise. kobo-gateway.dmg + the raw binary are built on macOS by
# build-kobo-gateway.yml and placed here by docker.yml before `docker build`
# runs.
COPY web/public/downloads/kobo-gateway-darwin-arm64 \
     ./web/public/downloads/kobo-gateway-darwin-arm64
COPY web/public/downloads/kobo-gateway.dmg \
     ./web/public/downloads/kobo-gateway.dmg

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 8000

ENTRYPOINT ["/app/bin/gateway"]
