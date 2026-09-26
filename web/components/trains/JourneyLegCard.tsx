import { Card } from '@/components/ui/card'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import StopStatusBadge from '@/components/trains/StopStatusBadge'
import type { LegDetail } from '@/lib/gen/trains/v1/trains_pb'

function formatTime(iso: string): string {
  if (!iso) return ''
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function StopRow({ stop }: { stop: LegDetail['stops'][number] }) {
  return (
    <li className="flex items-center justify-between gap-3 py-2">
      <div className="min-w-0">
        <p className="break-words font-medium text-fg">{stop.stopName || stop.stopId}</p>
        <p className="text-xs text-muted">
          {formatTime(stop.scheduledArrival || stop.scheduledDeparture)}
          {stop.platform ? ` · Platform ${stop.platform}` : ''}
        </p>
      </div>
      <div className="shrink-0">
        <StopStatusBadge status={stop.status} delaySeconds={stop.delaySeconds} />
      </div>
    </li>
  )
}

/** One boarded leg: header, cancellation banner, stops, and alerts. */
export default function JourneyLegCard({ leg }: { leg: LegDetail }) {
  return (
    <Card className="p-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="min-w-0 break-words font-semibold text-fg">
          {leg.routeShortName} {leg.tripShortName} → {leg.headsign}
        </h2>
        {leg.cancelled && <Badge variant="danger">Cancelled</Badge>}
      </div>

      {leg.alerts.length > 0 && (
        <ul className="mt-2 space-y-1">
          {leg.alerts.map((alert) => (
            <li key={alert.id}>
              <Alert tone="warn">
                <p className="font-medium">{alert.headerText}</p>
                {alert.descriptionText && <p className="mt-0.5 text-xs">{alert.descriptionText}</p>}
              </Alert>
            </li>
          ))}
        </ul>
      )}

      <ul className="mt-3 divide-y divide-border">
        {leg.stops.map((stop, i) => (
          <StopRow key={`${stop.stopId}-${i}`} stop={stop} />
        ))}
      </ul>
    </Card>
  )
}
