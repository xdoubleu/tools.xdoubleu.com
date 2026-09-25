import { Badge, type BadgeProps } from '@/components/ui/badge'

/**
 * "No live data" must never read as "on time", and skipped/cancelled stops
 * must stand out.
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
