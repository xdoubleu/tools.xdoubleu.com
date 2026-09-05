import TrainsClient from '@/components/trains/TrainsClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { TrainService } from '@/lib/gen/trains/v1/trains_pb'

export default async function TrainsPage() {
  const client = await createServerClient(TrainService)
  const feedInfo = await fetchOrNull(() => client.getFeedInfo({}))

  return (
    <SWRFallback fallback={feedInfo ? { [swrKeys.trainsFeedInfo]: feedInfo } : {}}>
      <TrainsClient />
    </SWRFallback>
  )
}
