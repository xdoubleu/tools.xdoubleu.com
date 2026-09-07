import { Card } from '@/components/ui/card'
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
      <div>
        <p className="font-medium text-fg">{stop.stopName || stop.stopId}</p>
        <p className="text-xs text-muted">
          {formatTime(stop.scheduledArrival || stop.scheduledDeparture)}
          {stop.platform ? ` · Platform ${stop.platform}` : ''}
        </p>
      </div>
      <StopStatusBadge status={stop.status} delaySeconds={stop.delaySeconds} />
    </li>
  )
}

/** One boarded leg of a journey: header, cancellation banner, its full stop
 * pattern, and any attached alerts — the "every leg with its stops"
 * requirement from issue #1394. */
export default function JourneyLegCard({ leg }: { leg: LegDetail }) {
  return (
    <Card className="p-4">
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-semibold text-fg">
          {leg.routeShortName} {leg.tripShortName} → {leg.headsign}
        </h2>
        {leg.cancelled && <Badge variant="danger">Cancelled</Badge>}
      </div>

      {leg.alerts.length > 0 && (
        <ul className="mt-2 space-y-1">
          {leg.alerts.map((alert) => (
            <li key={alert.id} className="rounded-md bg-warn/10 p-2 text-sm text-warn">
              <p className="font-medium">{alert.headerText}</p>
              {alert.descriptionText && <p className="mt-0.5 text-xs">{alert.descriptionText}</p>}
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
