import * as Sentry from '@sentry/nextjs'
import posthog from 'posthog-js'
import { getSentryDsn, getRelease, getPostHogKey, getPostHogHost } from './lib/env'
import { scrubPostHogEvent } from './lib/scrubUrl'

Sentry.init({
  dsn: getSentryDsn(),
  release: getRelease(),
  tracesSampleRate: 1.0,
  // v11 defaults to collecting everything; keep the restrictive v10 baseline.
  dataCollection: {
    userInfo: false,
    cookies: false,
    httpHeaders: {
      request: { deny: ['forwarded', '-ip', 'remote-', 'via', '-user'] },
      response: { deny: ['forwarded', '-ip', 'remote-', 'via', '-user'] }
    },
    httpBodies: [],
    urlQueryParams: { deny: ['forwarded', '-ip', 'remote-', 'via', '-user', 'token', 'code'] },
    genAI: { inputs: false, outputs: false },
    databaseQueryData: false,
    graphQL: { document: false, variables: false }
  }
})

// PostHog analytics + session replay for every family member, no consent
// gate. No key (local dev) leaves it uninitialized.
const postHogKey = getPostHogKey()
if (postHogKey) {
  posthog.init(postHogKey, {
    api_host: getPostHogHost(),
    person_profiles: 'identified_only',
    capture_pageview: 'history_change',
    capture_pageleave: true,
    autocapture: true,
    capture_dead_clicks: true,
    // Replay and autocapture skip `.ph-no-capture` (MFA secrets, recovery codes).
    before_send: scrubPostHogEvent
  })
}

export const onRouterTransitionStart = Sentry.captureRouterTransitionStart
