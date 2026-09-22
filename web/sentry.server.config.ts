import * as Sentry from '@sentry/nextjs'

Sentry.init({
  dsn: process.env.SENTRY_DSN_WEB,
  release: process.env.NEXT_PUBLIC_RELEASE || 'dev',
  tracesSampleRate: 1.0,
  ignoreErrors: ['The destination stream closed early.'],
  // Next.js emits `NextNodeServer.clientComponentLoading` as a synthetic
  // span whose duration is the accumulated sum of every client-module
  // require/chunk load since module-global counters were last reset —
  // counters shared across all requests and reported at whichever request
  // ends next, with pending chunk time included. The value therefore
  // measures nothing attributable to the request it lands on (p95s in the
  // hundreds of seconds; ADR-0011's historical classification reached the
  // same verdict). Real page-load latency is tracked per route and by the
  // web_vitals_seconds histogram (issue #1721).
  ignoreTransactions: ['NextNodeServer.clientComponentLoading']
})
