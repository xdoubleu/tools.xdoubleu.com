// Shared helpers for the admin observability UI.

// formatCount renders an integer count (number or bigint) with thousands
// separators.
export function formatCount(count: number | bigint): string {
  const n = typeof count === 'bigint' ? Number(count) : count
  return n.toLocaleString()
}

// formatDuration renders a millisecond duration compactly.
export function formatDuration(ms: number | bigint): string {
  const n = typeof ms === 'bigint' ? Number(ms) : ms
  if (n < 1000) return `${Math.round(n)} ms`
  if (n < 60_000) return `${(n / 1000).toFixed(1)} s`
  return `${(n / 60_000).toFixed(1)} min`
}

// AUTOMATED_ACTION_STALE_MINUTES is the "still running past a reasonable
// duration" threshold for the /monitoring/observability automated-actions
// table (issue #1442). global.automated_actions carries no per-routine
// expected-duration column to compare against, so this is a deliberate,
// judgment-call default rather than a derived value — most routines are
// short agent-driven tasks (a few minutes), so an hour with no close is
// already unusual enough to flag as overdue rather than merely slow.
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
