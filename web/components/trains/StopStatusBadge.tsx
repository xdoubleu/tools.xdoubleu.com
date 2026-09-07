import { Badge, type BadgeProps } from '@/components/ui/badge'

/**
 * The four states a stop call can honestly render as (issue #1394) — "no
 * live data" must never collapse into "on time", and a skipped/cancelled
 * stop must always read as distinct and prominent.
 */
function labelFor(
  status: string,
  delaySeconds: number
): { text: string; variant: BadgeProps['variant'] } {
  switch (status) {
    case 'on_time':
      return { text: 'On time', variant: 'success' }
    case 'delayed': {
      const minutes = Math.round(Math.abs(delaySeconds) / 60)
      return { text: `Delayed ${minutes}m`, variant: 'warn' }
    }
    case 'skipped':
      return { text: 'Skipped', variant: 'danger' }
    case 'cancelled':
      return { text: 'Cancelled', variant: 'danger' }
    case 'unknown':
    default:
      return { text: 'No live data', variant: 'secondary' }
  }
}

export default function StopStatusBadge({
  status,
  delaySeconds
}: {
  status: string
  delaySeconds: number
}) {
  const { text, variant } = labelFor(status, delaySeconds)
  return <Badge variant={variant}>{text}</Badge>
}
