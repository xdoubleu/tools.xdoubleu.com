import EditFeedClient from './EditFeedClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ICSProxyService } from '@/lib/gen/icsproxy/v1/proxy_pb'

interface EditFeedPageProps {
  params: Promise<{ token: string }>
}

export default async function EditFeedPage({ params }: EditFeedPageProps) {
  const { token } = await params
  const client = await createServerClient(ICSProxyService)
  const config = await fetchOrNull(() => client.getConfig({ token }))

  return (
    <SWRFallback fallback={config ? { [swrKeys.icsConfig(token)]: config } : {}}>
      <EditFeedClient token={token} />
    </SWRFallback>
  )
}
