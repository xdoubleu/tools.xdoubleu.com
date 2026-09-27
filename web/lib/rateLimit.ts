// In-memory fixed-window rate limiter for public route handlers. Per process,
// which is enough for the single standalone web server.

const MAX_TRACKED_KEYS = 10_000

export interface RateLimiter {
  limited(key: string, now?: number): boolean
}

export function createRateLimiter(limit: number, windowMs: number): RateLimiter {
  const hits = new Map<string, { windowStart: number; count: number }>()
  return {
    limited(key, now = Date.now()) {
      const entry = hits.get(key)
      if (!entry || now - entry.windowStart >= windowMs) {
        if (hits.size >= MAX_TRACKED_KEYS) hits.clear()
        hits.set(key, { windowStart: now, count: 1 })
        return false
      }
      entry.count++
      return entry.count > limit
    }
  }
}

// kamal-proxy appends the peer address last; earlier hops are client-supplied.
export function clientIP(request: Request): string {
  const forwarded = request.headers.get('x-forwarded-for') ?? ''
  return forwarded.split(',').pop()?.trim() || 'unknown'
}
