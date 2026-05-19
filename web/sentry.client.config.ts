import * as Sentry from '@sentry/nextjs'
import { getSentryDsn } from './lib/env'

Sentry.init({
  dsn: getSentryDsn(),
  debug: process.env.NODE_ENV === 'development',
  tracesSampleRate: 1.0
})
