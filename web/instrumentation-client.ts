import * as Sentry from '@sentry/nextjs'
import { getSentryDsn, getRelease } from './lib/env'

Sentry.init({
  dsn: getSentryDsn(),
  release: getRelease(),
  tracesSampleRate: 1.0
})

export const onRouterTransitionStart = Sentry.captureRouterTransitionStart
