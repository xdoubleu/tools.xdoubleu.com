/**
 * @jest-environment node
 */
// next/server needs the real Request/Response globals, which jsdom lacks.
import { middleware } from '@/middleware'

function connectSrc(): string {
  const csp = middleware().headers.get('Content-Security-Policy') ?? ''
  return csp.split('; ').find((d) => d.startsWith('connect-src ')) ?? ''
}

describe('middleware CSP', () => {
  it('allows the local kobo-gateway origin', () => {
    // Without this the books page can't reach the gateway at all (#960).
    expect(connectSrc()).toContain('https://127.0.0.1:41132')
  })

  it('allows Cloudflare R2 for browser-side book file uploads', () => {
    // The upload PUT goes straight to an R2 presigned URL (#1572).
    expect(connectSrc()).toContain('https://*.r2.cloudflarestorage.com')
  })

  it('includes the API origin when configured', () => {
    process.env.API_URL = 'https://example.com/api'
    expect(connectSrc()).toContain('https://example.com/api')
    delete process.env.API_URL
  })
})
