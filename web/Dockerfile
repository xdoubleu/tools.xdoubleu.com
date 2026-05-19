FROM node:22-alpine AS builder

WORKDIR /app

RUN corepack enable

COPY web/package.json web/yarn.lock web/.yarnrc.yml ./

RUN yarn install --immutable

COPY web/app app
COPY web/components components
COPY web/hooks hooks
COPY web/lib lib
COPY web/next.config.ts web/tsconfig.json web/postcss.config.js \
     web/tailwind.config.ts web/sentry.client.config.ts \
     web/sentry.edge.config.ts web/sentry.server.config.ts \
     web/next-env.d.ts ./

ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}

RUN yarn build

FROM node:22-alpine AS server

WORKDIR /app

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

EXPOSE 3000
CMD ["node", "server.js"]
