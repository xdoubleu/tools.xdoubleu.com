import * as Sentry from '@sentry/nextjs'
import posthog from 'posthog-js'
import { getSentryDsn, getRelease, getPostHogKey, getPostHogHost } from './lib/env'

Sentry.init({
  dsn: getSentryDsn(),
  release: getRelease(),
  tracesSampleRate: 1.0
})

// Product analytics + session replay (root AGENTS.md's PostHog decision) —
// autocapture and session recording on by default for every family member,
// no opt-in/consent gate. A missing key (e.g. local dev) leaves PostHog
// uninitialized rather than erroring.
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
