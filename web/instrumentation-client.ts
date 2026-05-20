import * as Sentry from '@sentry/nextjs'
import { getSentryDsn } from './lib/env'

Sentry.init({
  dsn: getSentryDsn(),
  debug: true,
  tracesSampleRate: 1.0,
  enableLogs: true
})
