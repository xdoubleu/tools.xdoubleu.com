import Link from 'next/link'
import { Card, interactiveCardClass } from '@/components/ui/card'
import { cn } from '@/lib/cn'
import type { Journey } from '@/lib/gen/trains/v1/trains_pb'

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function formatDuration(departureIso: string, arrivalIso: string): string {
  const minutes = Math.round(
    (new Date(arrivalIso).getTime() - new Date(departureIso).getTime()) / 60000
  )
  const hours = Math.floor(minutes / 60)
  const remainder = minutes % 60
  return hours > 0 ? `${hours}h ${remainder}m` : `${remainder}m`
}

function transfersLabel(transfers: number): string {
  if (transfers === 0) return 'Direct'
  return transfers === 1 ? '1 change' : `${transfers} changes`
}

export function JourneyRow({ journey }: { journey: Journey }) {
  const trains = journey.legs.map((leg) => leg.tripShortName).join(', ')

  if (!journey.journeyId) {
    // Defensive fallback — the api always sets journey_id, but a row with
    // none can't open a detail page.
    return (
      <Card className="p-4">
        <JourneyRowContent journey={journey} trains={trains} />
      </Card>
    )
  }

  return (
    <Link
      href={`/trains/${encodeURIComponent(journey.journeyId)}`}
      className={cn(interactiveCardClass, 'block p-4')}
    >
      <JourneyRowContent journey={journey} trains={trains} />
    </Link>
  )
}

function JourneyRowContent({ journey, trains }: { journey: Journey; trains: string }) {
  return (
    <>
      <div className="flex items-center justify-between gap-3">
        <div className="text-fg">
          <span className="font-semibold">{formatTime(journey.departureTime)}</span>
          <span className="mx-1.5 text-muted">→</span>
          <span className="font-semibold">{formatTime(journey.arrivalTime)}</span>
        </div>
        <span className="shrink-0 text-sm text-muted">
          {formatDuration(journey.departureTime, journey.arrivalTime)}
        </span>
      </div>
      <p className="mt-1 text-sm text-muted">
        {transfersLabel(journey.transfers)} · {trains}
      </p>
    </>
  )
}

interface JourneyResultsProps {
  ready: boolean
  isLoading: boolean
  error: unknown
  feedImported: boolean
  journeys: Journey[]
}

/** The route overview list, plus the empty/degraded states called out in issue #1392. */
export default function JourneyResults({
  ready,
  isLoading,
  error,
  feedImported,
  journeys
}: JourneyResultsProps) {
  if (!ready) return null
  if (isLoading) return <p className="text-muted">Loading…</p>
  if (error) return <p className="text-danger">Failed to load journeys.</p>
  if (!feedImported) {
    return <p className="text-muted">The timetable hasn&apos;t been imported yet.</p>
  }
  if (journeys.length === 0) {
    return <p className="text-muted">No trains found for this station pair and time.</p>
  }

  return (
    <div className="space-y-3">
      {journeys.map((journey) => (
        <JourneyRow key={`${journey.departureTime}-${journey.arrivalTime}`} journey={journey} />
      ))}
    </div>
  )
}
