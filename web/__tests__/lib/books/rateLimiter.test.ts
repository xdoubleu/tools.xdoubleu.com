import { createRateLimiter } from '@/lib/books/rateLimiter'

describe('createRateLimiter', () => {
  it('allows burst-sized acquires immediately', async () => {
    const acquire = createRateLimiter(10, 2)
    const start = Date.now()
    await acquire()
    await acquire()
    expect(Date.now() - start).toBeLessThan(50)
  })

  it('delays once the burst is exhausted', async () => {
    const acquire = createRateLimiter(10, 1)
    await acquire()
    const start = Date.now()
    await acquire()
    expect(Date.now() - start).toBeGreaterThanOrEqual(90)
  })
})
