import { cn } from '@/lib/cn'

/**
 * "No live data" must never read as "on time", and skipped/cancelled stops
 * must stand out.
 */
function labelFor(status: string, delaySeconds: number): { text: string; tone: Tone } {
  switch (status) {
    case 'on_time':
      return { text: 'On time', tone: 'success' }
    case 'delayed': {
      const minutes = Math.round(Math.abs(delaySeconds) / 60)
      return { text: `Delayed ${minutes}m`, tone: 'warn' }
    }
    case 'skipped':
      return { text: 'Skipped', tone: 'danger' }
    case 'cancelled':
      return { text: 'Cancelled', tone: 'danger' }
    case 'unknown':
    default:
      return { text: 'No live data', tone: 'muted' }
  }
}

type Tone = 'success' | 'warn' | 'danger' | 'muted'

const DOT_CLASS: Record<Tone, string> = {
  success: 'bg-success',
  warn: 'bg-warn',
  danger: 'bg-danger',
  muted: 'bg-muted'
}

export default function StopStatusBadge({
  status,
  delaySeconds
}: {
  status: string
  delaySeconds: number
}) {
  const { text, tone } = labelFor(status, delaySeconds)
  return (
    <span className="inline-flex cursor-default items-center gap-1.5 text-xs font-medium">
      <span aria-hidden className={cn('size-2 rounded-full', DOT_CLASS[tone])} />
      {text}
    </span>
  )
}
