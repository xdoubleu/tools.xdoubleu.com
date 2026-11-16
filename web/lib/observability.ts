import { formatNumber } from '@/lib/format'

export function formatCount(count: number | bigint): string {
  const n = typeof count === 'bigint' ? Number(count) : count
  return formatNumber(n)
}

export function formatDuration(ms: number | bigint): string {
  const n = typeof ms === 'bigint' ? Number(ms) : ms
  if (n < 1000) return `${Math.round(n)} ms`
  if (n < 60_000) return `${(n / 1000).toFixed(1)} s`
  return `${(n / 60_000).toFixed(1)} min`
}
