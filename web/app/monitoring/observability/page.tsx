import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'
import { PageContainer } from '@/components/ui/page-container'
import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

export default async function MonitoringObservabilityPage() {
  const client = await createServerClient(ObservabilityService)
  const automatedActions = await fetchOrNull(() => client.getAutomatedActions({}))

  const fallback: Record<string, unknown> = {}
  if (automatedActions) fallback[swrKeys.monitoringAutomatedActions] = automatedActions

  return (
    <PageContainer className="p-6">
      <SWRFallback fallback={fallback}>
        <h1 className="mb-6 text-3xl font-bold">Observability</h1>
        <ObservabilityClient />
      </SWRFallback>
    </PageContainer>
  )
}
