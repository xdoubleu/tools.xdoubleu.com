FROM node:24-alpine AS builder

ARG RELEASE=dev

WORKDIR /app

COPY web/package.json web/package-lock.json ./

RUN npm ci

COPY web/app app
COPY web/components components
COPY web/hooks hooks
COPY web/lib lib
COPY web/next.config.ts web/tsconfig.json web/postcss.config.js \
     web/tailwind.config.ts web/instrumentation-client.ts \
     web/sentry.edge.config.ts web/sentry.server.config.ts \
     web/next-env.d.ts ./

ENV RELEASE=${RELEASE}
RUN npm run build

FROM node:24-alpine AS server

ARG RELEASE=dev
ENV RELEASE=${RELEASE}

WORKDIR /app

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 3000
CMD ["node", "server.js"]
