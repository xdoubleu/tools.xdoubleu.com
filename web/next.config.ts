import type { NextConfig } from 'next'
import { withSentryConfig } from '@sentry/nextjs'

const securityHeaders = [
  { key: 'Strict-Transport-Security', value: 'max-age=31536000; includeSubDomains' },
  { key: 'X-Frame-Options', value: 'DENY' },
  { key: 'X-Content-Type-Options', value: 'nosniff' },
  { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' }
]

const nextConfig: NextConfig = {
  output: 'standalone',
  poweredByHeader: false,
  // Steam (achievement/game icons) and Open Library (book covers) serve images
  // from rotating, sometimes http-only CDN hosts. Serve them as-is instead of
  // routing through the Next optimizer, which would otherwise block
  // un-whitelisted hosts.
  images: {
    unoptimized: true
  },
  env: {
    NEXT_PUBLIC_RELEASE: process.env.RELEASE || 'dev'
  },
  async headers() {
    return [{ source: '/(.*)', headers: securityHeaders }]
  }
}

export default withSentryConfig(nextConfig, {
  silent: !process.env.CI,
  org: process.env.SENTRY_ORG,
  project: process.env.SENTRY_PROJECT,
  tunnelRoute: '/sentry-tunnel',
  authToken: process.env.SENTRY_AUTH_TOKEN
})
