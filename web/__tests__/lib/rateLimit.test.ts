/**
 * @jest-environment node
 */
import { clientIP, createRateLimiter } from '@/lib/rateLimit'

describe('createRateLimiter', () => {
  it('allows up to the limit per window, then resets', () => {
    const limiter = createRateLimiter(2, 1000)
    expect(limiter.limited('a', 0)).toBe(false)
    expect(limiter.limited('a', 10)).toBe(false)
    expect(limiter.limited('a', 20)).toBe(true)
    expect(limiter.limited('b', 20)).toBe(false)
    expect(limiter.limited('a', 1000)).toBe(false)
  })

  it('stays bounded by clearing when too many keys are tracked', () => {
    const limiter = createRateLimiter(1, 1000)
    for (let i = 0; i < 10_000; i++) limiter.limited(`k${i}`, 0)
    expect(limiter.limited('k0', 0)).toBe(true)
    limiter.limited('new', 0)
    expect(limiter.limited('k0', 0)).toBe(false)
  })
})

describe('clientIP', () => {
  it('uses the last X-Forwarded-For hop', () => {
    const request = new Request('http://localhost', {
      headers: { 'x-forwarded-for': '1.1.1.1, 2.2.2.2' }
    })
    expect(clientIP(request)).toBe('2.2.2.2')
  })

  it('falls back when the header is missing', () => {
    expect(clientIP(new Request('http://localhost'))).toBe('unknown')
  })
})
