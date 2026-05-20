import type { NextConfig } from 'next'
import { withSentryConfig } from '@sentry/nextjs'

const nextConfig: NextConfig = {
  output: 'standalone',
  env: {
    NEXT_PUBLIC_RELEASE: process.env.RELEASE || 'dev'
  }
}

export default withSentryConfig(nextConfig, {
  silent: !process.env.CI,
  org: process.env.SENTRY_ORG,
  project: process.env.SENTRY_PROJECT,
  tunnelRoute: "/sentry-tunnel",
  authToken: process.env.SENTRY_AUTH_TOKEN
})
