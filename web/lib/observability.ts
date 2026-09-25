export function formatCount(count: number | bigint): string {
  const n = typeof count === 'bigint' ? Number(count) : count
  return n.toLocaleString()
}

export function formatDuration(ms: number | bigint): string {
  const n = typeof ms === 'bigint' ? Number(ms) : ms
  if (n < 1000) return `${Math.round(n)} ms`
  if (n < 60_000) return `${(n / 1000).toFixed(1)} s`
  return `${(n / 60_000).toFixed(1)} min`
}

// Judgment-call threshold: routines are usually minutes, and no per-routine
// expected duration exists.
export const AUTOMATED_ACTION_STALE_MINUTES = 60

// isAutomatedActionStale reports whether a still-open run (empty finishedAt)
// has been open longer than AUTOMATED_ACTION_STALE_MINUTES.
export function isAutomatedActionStale(firedAt: string, finishedAt: string): boolean {
  if (finishedAt) return false
  const fired = new Date(firedAt)
  if (Number.isNaN(fired.getTime())) return false
  const elapsedMs = Date.now() - fired.getTime()
  return elapsedMs > AUTOMATED_ACTION_STALE_MINUTES * 60_000
}
