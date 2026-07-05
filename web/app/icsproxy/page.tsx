import FeedsListClient from '@/components/icsproxy/FeedsListClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ICSProxyService } from '@/lib/gen/icsproxy/v1/proxy_pb'

export default async function ICSProxyPage() {
  const client = await createServerClient(ICSProxyService)
  const feeds = await fetchOrNull(() => client.listConfigs({}))

  return (
    <SWRFallback fallback={feeds ? { [swrKeys.icsFeeds]: feeds } : {}}>
      <FeedsListClient />
    </SWRFallback>
  )
}
