import * as Sentry from '@sentry/nextjs'

Sentry.init({
  dsn: process.env.SENTRY_DSN_WEB,
  release: process.env.NEXT_PUBLIC_RELEASE || 'dev',
  tracesSampleRate: 1.0,
  ignoreErrors: ['The destination stream closed early.'],
  // A synthetic span summing chunk loads across requests; its duration isn't
  // attributable to the request it lands on.
  ignoreTransactions: ['NextNodeServer.clientComponentLoading']
})
