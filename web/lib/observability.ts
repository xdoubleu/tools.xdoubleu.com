// Shared helpers for the admin observability UI.

// formatCount renders an integer count (number or bigint) with thousands
// separators.
export function formatCount(count: number | bigint): string {
  const n = typeof count === 'bigint' ? Number(count) : count
  return n.toLocaleString()
}
