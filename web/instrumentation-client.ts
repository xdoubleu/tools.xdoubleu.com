import * as Sentry from '@sentry/nextjs'
import posthog from 'posthog-js'
import { getSentryDsn, getRelease, getPostHogKey, getPostHogHost } from './lib/env'

Sentry.init({
  dsn: getSentryDsn(),
  release: getRelease(),
  tracesSampleRate: 1.0
})

// PostHog analytics + session replay for every family member, no consent
// gate. No key (local dev) leaves it uninitialized.
const postHogKey = getPostHogKey()
if (postHogKey) {
  posthog.init(postHogKey, {
    api_host: getPostHogHost(),
    person_profiles: 'identified_only',
    capture_pageview: true,
    capture_pageleave: true
  })
}

export const onRouterTransitionStart = Sentry.captureRouterTransitionStart
