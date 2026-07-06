// Shared helpers for the admin observability dashboard.

// CATEGORICAL_PALETTE is a CVD-safe ordered hue set (validated with the
// dataviz palette validator). Series colours are assigned by fixed index —
// never cycled — and a 7th+ series folds into an "Other" bucket upstream.
export const CATEGORICAL_PALETTE = [
  '#2a78d6', // blue
  '#1baf7a', // aqua
  '#eda100', // yellow
  '#008300', // green
  '#4a3aa7', // violet
  '#e34948', // red
  '#e87ba4' // magenta
] as const

// OTHER_COLOR paints the aggregated "Other" bucket (muted, non-identity).
export const OTHER_COLOR = '#9aa0a6'

// chartTooltipStyle themes the recharts tooltip with the app's CSS variables
// so it tracks light/dark mode automatically.
export const chartTooltipStyle = {
  backgroundColor: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: '0.75rem',
  color: 'var(--color-fg)'
} as const

// formatBytes renders a byte count (number or protobuf bigint) as a
// human-readable size.
export function formatBytes(bytes: number | bigint): string {
  const n = typeof bytes === 'bigint' ? Number(bytes) : bytes
  if (n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const exp = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1)
  const value = n / Math.pow(1024, exp)
  const decimals = value >= 100 || exp === 0 ? 0 : 1
  return `${value.toFixed(decimals)} ${units[exp]}`
}

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

// successRate returns the fraction of successful runs as a 0–100 percentage.
export function successRate(total: number | bigint, failed: number | bigint): number {
  const t = typeof total === 'bigint' ? Number(total) : total
  const f = typeof failed === 'bigint' ? Number(failed) : failed
  if (t <= 0) return 100
  return Math.round(((t - f) / t) * 100)
}
