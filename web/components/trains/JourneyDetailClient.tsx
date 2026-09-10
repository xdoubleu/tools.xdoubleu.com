'use client'

import { PageContainer } from '@/components/ui/page-container'
import { Badge } from '@/components/ui/badge'
import JourneyLegCard from '@/components/trains/JourneyLegCard'
import JourneyAlternativePanel from '@/components/trains/JourneyAlternativePanel'
import { useJourneyDetail } from '@/hooks/useTrains'
import { useJourneyLive } from '@/lib/trains/journeySocket'

export default function JourneyDetailClient({ journeyId }: { journeyId: string }) {
  const { data, error, isLoading, mutate } = useJourneyDetail(journeyId)
  const { connected, pushedDetail } = useJourneyLive(journeyId, () => mutate())

  if (isLoading) return <p className="text-muted">Loading…</p>
  if (error) return <p className="text-danger">Failed to load journey.</p>

  // A websocket push is the server's latest rebuild of this journey and
  // takes priority over the SWR-cached fetch the moment one arrives, so a
  // delay/cancellation shows up without waiting on a refetch.
  const journey = pushedDetail ?? data?.journey
  if (!journey) return <p className="text-danger">Failed to load journey.</p>

  return (
    <PageContainer size="narrow" className="p-6">
      <div className="mb-6 flex items-center justify-between gap-3">
        <h1 className="text-3xl font-bold">Journey</h1>
        <Badge variant={connected ? 'success' : 'secondary'}>
          {connected ? 'Live' : 'Reconnecting…'}
        </Badge>
      </div>

      {journey.alternative && (
        <div className="mb-4">
          <JourneyAlternativePanel alternative={journey.alternative} />
        </div>
      )}

      <div className="space-y-3">
        {journey.legs.map((leg, i) => (
          <JourneyLegCard key={`${leg.tripShortName}-${i}`} leg={leg} />
        ))}
      </div>
    </PageContainer>
  )
}
