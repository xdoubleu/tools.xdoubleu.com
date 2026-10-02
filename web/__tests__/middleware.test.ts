/**
 * @jest-environment node
 */
// next/server needs the real Request/Response globals, which jsdom lacks.
import { NextRequest } from 'next/server'
import { config, middleware } from '@/middleware'

function run() {
  return middleware(new NextRequest('https://tools.example.com/'))
}

function cspDirective(name: string): string {
  const csp = run().headers.get('Content-Security-Policy') ?? ''
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

  it('allows inline scripts only by a fresh per-request nonce', () => {
    const scriptSrc = cspDirective('script-src')
    expect(scriptSrc).not.toContain("'unsafe-inline'")
    expect(scriptSrc).toContain("'strict-dynamic'")
    expect(scriptSrc).toMatch(/'nonce-[A-Za-z0-9+/=]{20,}'/)
    expect(cspDirective('script-src')).not.toEqual(scriptSrc)
  })

  it('forwards the nonce and CSP to rendering via request headers', () => {
    const res = run()
    const nonce = res.headers.get('x-middleware-request-x-nonce')
    expect(nonce).toBeTruthy()
    expect(res.headers.get('Content-Security-Policy')).toContain(`'nonce-${nonce}'`)
    expect(res.headers.get('x-middleware-request-content-security-policy')).toEqual(
      res.headers.get('Content-Security-Policy')
    )
  })

  it('allows the foliate-js reader its blob: frames, stylesheets and fonts', () => {
    expect(cspDirective('frame-src')).toContain('blob:')
    expect(cspDirective('style-src')).toContain('blob:')
    expect(cspDirective('font-src')).toBe("font-src 'self' blob:")
  })

  it('skips the static foliate-js files', () => {
    const matcher = new RegExp(`^${config.matcher[0]}$`)
    expect(matcher.test('/foliate-js/vendor/pdfjs/pdf.worker.mjs')).toBe(false)
    expect(matcher.test('/books/ub-1/read')).toBe(true)
  })

  it('adds unsafe-eval only in development', () => {
    expect(cspDirective('script-src')).not.toContain("'unsafe-eval'")
    const env = process.env as Record<string, string | undefined>
    const original = env.NODE_ENV
    env.NODE_ENV = 'development'
    try {
      expect(cspDirective('script-src')).toContain("'unsafe-eval'")
    } finally {
      env.NODE_ENV = original
    }
  })
})
