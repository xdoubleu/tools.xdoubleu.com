import { NextResponse, type NextRequest } from 'next/server'

import { GATEWAY_URL } from '@/lib/books/gatewayClient'

export function middleware(request: NextRequest) {
  // Next.js reads the nonce from the request's CSP header and stamps it on its
  // own scripts; the layout reads x-nonce for its inline ones.
  const nonce = btoa(crypto.randomUUID())

  // GATEWAY_URL: loopback kobo-gateway. R2: direct presigned upload PUTs.
  const connectSrc = ["'self'", '*.sentry.io', 'https://*.r2.cloudflarestorage.com', GATEWAY_URL]
  if (process.env.API_URL) connectSrc.push(process.env.API_URL)

  // 'strict-dynamic' trusts scripts that nonced scripts load (chunks, PostHog
  // extensions); the host sources are the CSP2 fallback.
  const scriptSrc = ["'self'", `'nonce-${nonce}'`, "'strict-dynamic'"]
  // React uses eval for dev-only error stacks.
  if (process.env.NODE_ENV === 'development') scriptSrc.push("'unsafe-eval'")

  // PostHog captures to POSTHOG_HOST and loads config from its -assets sibling.
  const postHogHost = process.env.POSTHOG_HOST
  if (postHogHost) {
    const postHogAssetsHost = postHogHost.replace(
      /^https:\/\/([a-z-]+)\.i\./,
      'https://$1-assets.i.'
    )
    connectSrc.push(postHogHost, postHogAssetsHost)
    scriptSrc.push(postHogAssetsHost)
  }

  const csp = [
    "default-src 'self'",
    `script-src ${scriptSrc.join(' ')}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob: https:",
    `connect-src ${connectSrc.join(' ')}`,
    // PDF previews use an <iframe> on a presigned R2 URL.
    "frame-src 'self' https://*.r2.cloudflarestorage.com",
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "object-src 'none'"
  ].join('; ')

  const requestHeaders = new Headers(request.headers)
  requestHeaders.set('x-nonce', nonce)
  requestHeaders.set('Content-Security-Policy', csp)

  const response = NextResponse.next({ request: { headers: requestHeaders } })
  response.headers.set('Content-Security-Policy', csp)
  return response
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico).*)']
}
