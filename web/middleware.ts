import { NextResponse } from 'next/server'

import { GATEWAY_URL } from '@/lib/books/gatewayClient'

export function middleware() {
  const response = NextResponse.next()

  // GATEWAY_URL: the books page talks to the local kobo-gateway helper over
  // loopback HTTPS, which 'self' doesn't cover.
  const connectSrc = ["'self'", '*.sentry.io', GATEWAY_URL]
  if (process.env.API_URL) connectSrc.push(process.env.API_URL)

  const csp = [
    "default-src 'self'",
    "script-src 'self' 'unsafe-inline'",
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob: https:",
    `connect-src ${connectSrc.join(' ')}`,
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "form-action 'self'"
  ].join('; ')

  response.headers.set('Content-Security-Policy', csp)
  return response
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico).*)']
}
