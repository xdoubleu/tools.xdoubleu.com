import * as Sentry from '@sentry/nextjs'

Sentry.init({
  dsn: process.env.SENTRY_DSN,
  release: process.env.NEXT_PUBLIC_RELEASE || 'dev',
  tracesSampleRate: 1.0
})
