import Link from 'next/link'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { JourneyRow } from '@/components/trains/JourneyResults'
import type { JourneyAlternative } from '@/lib/gen/trains/v1/trains_pb'

/**
 * Inline re-plan surfaced on the live journey page when a positive realtime
 * signal shows the planned journey no longer works (issue #1395): the reason
 * it broke, the re-planned itinerary in the same overview-row shape as a
 * search result, and a confirm action that switches the page to it. Absence
 * of live data never produces one — that decision is made server-side.
 */
export default function JourneyAlternativePanel({
  alternative
}: {
  alternative: JourneyAlternative
}) {
  const journey = alternative.journey

  return (
    <Card className="border-warn/40 bg-warn/5 p-4">
      <div className="flex items-center gap-2">
        <Badge variant="warn">Journey disrupted</Badge>
      </div>
      <p className="mt-2 text-sm text-fg">{alternative.reason}</p>

      {journey ? (
        <>
          <p className="mt-3 text-xs font-medium text-muted">
            Alternative from {alternative.fromStopName}
          </p>
          <div className="mt-1">
            <JourneyRow journey={journey} />
          </div>
          {journey.journeyId && (
            <Button asChild variant="default" className="mt-3 w-full sm:w-auto">
              <Link href={`/trains/${encodeURIComponent(journey.journeyId)}`}>
                Switch to this journey
              </Link>
            </Button>
          )}
        </>
      ) : (
        <p className="mt-3 text-sm text-muted">
          No alternative route found from {alternative.fromStopName} right now.
        </p>
      )}
    </Card>
  )
}
