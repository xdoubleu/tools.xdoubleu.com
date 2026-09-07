import JourneyDetailClient from '@/components/trains/JourneyDetailClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { TrainService } from '@/lib/gen/trains/v1/trains_pb'

export default async function JourneyDetailPage({
  params
}: {
  params: Promise<{ journeyId: string }>
}) {
  const { journeyId } = await params
  const client = await createServerClient(TrainService)
  const detail = await fetchOrNull(() => client.getJourneyDetail({ journeyId }))

  return (
    <SWRFallback
      fallback={{}}
      keyed={detail ? [[swrKeys.trainsJourneyDetail(journeyId), detail]] : []}
    >
      <JourneyDetailClient journeyId={journeyId} />
    </SWRFallback>
  )
}
