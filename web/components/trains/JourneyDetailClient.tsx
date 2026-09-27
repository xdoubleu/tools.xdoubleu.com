'use client'

import { PageContainer } from '@/components/ui/page-container'
import { Badge } from '@/components/ui/badge'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'
import JourneyLegCard from '@/components/trains/JourneyLegCard'
import JourneyAlternativePanel from '@/components/trains/JourneyAlternativePanel'
import { useJourneyDetail } from '@/hooks/useTrains'
import { useJourneyLive } from '@/hooks/useJourneySocket'

export default function JourneyDetailClient({ journeyId }: { journeyId: string }) {
  const { data, error, isLoading, mutate } = useJourneyDetail(journeyId)
  const { connected, pushedDetail } = useJourneyLive(journeyId, () => mutate())

  if (isLoading) return <LoadingState label="journey" />
  if (error) return <ErrorState what="journey" />

  // A websocket push is the latest rebuild and wins over the SWR fetch.
  const journey = pushedDetail ?? data?.journey
  if (!journey) return <ErrorState what="journey" />

  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Journey"
        breadcrumb={[{ label: 'Trains', href: '/trains' }, { label: 'Journey' }]}
        actions={
          <Badge variant={connected ? 'success' : 'secondary'}>
            {connected ? 'Live' : 'Reconnecting…'}
          </Badge>
        }
      />

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
