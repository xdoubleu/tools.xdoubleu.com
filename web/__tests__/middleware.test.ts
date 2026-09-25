/**
 * @jest-environment node
 */
// next/server needs the real Request/Response globals, which jsdom lacks.
import { middleware } from '@/middleware'

function cspDirective(name: string): string {
  const csp = middleware().headers.get('Content-Security-Policy') ?? ''
  return csp.split('; ').find((d) => d.startsWith(`${name} `)) ?? ''
}

function connectSrc(): string {
  return cspDirective('connect-src')
}

describe('middleware CSP', () => {
  it('allows the local kobo-gateway origin', () => {
    expect(connectSrc()).toContain('https://127.0.0.1:41132')
  })

  it('allows Cloudflare R2 for browser-side book file uploads', () => {
    expect(connectSrc()).toContain('https://*.r2.cloudflarestorage.com')
  })

  it('includes the API origin when configured', () => {
    process.env.API_URL = 'https://example.com/api'
    expect(connectSrc()).toContain('https://example.com/api')
    delete process.env.API_URL
  })

  it('allows PostHog Cloud EU when POSTHOG_HOST is configured (#1857)', () => {
    // The SDK also loads config from the host's -assets sibling.
    process.env.POSTHOG_HOST = 'https://eu.i.posthog.com'
    expect(connectSrc()).toContain('https://eu.i.posthog.com')
    expect(connectSrc()).toContain('https://eu-assets.i.posthog.com')
    expect(cspDirective('script-src')).toContain('https://eu-assets.i.posthog.com')
    delete process.env.POSTHOG_HOST
  })

  it('adds no PostHog directives when POSTHOG_HOST is unset', () => {
    expect(connectSrc()).not.toContain('posthog.com')
    expect(cspDirective('script-src')).not.toContain('posthog.com')
  })
})
