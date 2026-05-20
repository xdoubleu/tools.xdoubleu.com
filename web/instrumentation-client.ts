import * as Sentry from '@sentry/nextjs'
import { getSentryDsn, getRelease } from './lib/env'

Sentry.init({
  dsn: getSentryDsn(),
  release: getRelease(),
  debug: true,
  tracesSampleRate: 1.0,
  enableLogs: true
})

export const onRouterTransitionStart = Sentry.captureRouterTransitionStart
