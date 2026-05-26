FROM node:24-alpine AS builder

ARG RELEASE=dev

WORKDIR /app

RUN corepack enable

COPY web/package.json web/yarn.lock web/.yarnrc.yml ./

RUN yarn install --immutable

COPY web/app app
COPY web/components components
COPY web/hooks hooks
COPY web/lib lib
COPY web/next.config.ts web/tsconfig.json web/postcss.config.js \
     web/tailwind.config.ts web/instrumentation-client.ts \
     web/sentry.edge.config.ts web/sentry.server.config.ts \
     web/next-env.d.ts ./

ENV RELEASE=${RELEASE}
RUN yarn build

FROM node:24-alpine AS server

ARG RELEASE=dev
ENV RELEASE=${RELEASE}

WORKDIR /app

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

EXPOSE 3000
CMD ["node", "server.js"]
