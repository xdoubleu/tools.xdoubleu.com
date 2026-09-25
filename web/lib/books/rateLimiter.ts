// Token bucket (golang.org/x/time/rate semantics) for pacing requests below a
// server rate limit.
export function createRateLimiter(ratePerSecond: number, burst: number) {
  let tokens = burst
  let last = Date.now()

  return async function acquire(): Promise<void> {
    const now = Date.now()
    tokens = Math.min(burst, tokens + ((now - last) / 1000) * ratePerSecond)
    last = now

    if (tokens < 1) {
      const waitMs = ((1 - tokens) / ratePerSecond) * 1000
      await new Promise((resolve) => setTimeout(resolve, waitMs))
      tokens = 0
      last = Date.now()
    } else {
      tokens -= 1
    }
  }
}
