import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'
import { PageContainer } from '@/components/ui/page-container'
import MonitoringSettingsClient from '@/components/monitoring/MonitoringSettingsClient'

export default async function MonitoringConnectionsPage() {
  const client = await createServerClient(ObservabilityService)
  const oauthConnections = await fetchOrNull(() => client.listOAuthConnections({}))

  const fallback: Record<string, unknown> = {}
  if (oauthConnections) fallback[swrKeys.monitoringOAuthConnections] = oauthConnections

  return (
    <PageContainer className="p-6">
      <SWRFallback fallback={fallback}>
        <h1 className="mb-6 text-3xl font-bold">Connections</h1>
        <MonitoringSettingsClient />
      </SWRFallback>
    </PageContainer>
  )
}
