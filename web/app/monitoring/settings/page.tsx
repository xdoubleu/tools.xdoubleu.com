import Link from 'next/link'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'
import { PageContainer } from '@/components/ui/page-container'
import MonitoringSettingsClient from '@/components/monitoring/MonitoringSettingsClient'

export default async function MonitoringSettingsPage() {
  const client = await createServerClient(ObservabilityService)
  const [notificationSettings, oauthConnections] = await Promise.all([
    fetchOrNull(() => client.getNotificationSettings({})),
    fetchOrNull(() => client.listOAuthConnections({}))
  ])

  const fallback: Record<string, unknown> = {}
  if (notificationSettings) fallback[swrKeys.monitoringNotificationSettings] = notificationSettings
  if (oauthConnections) fallback[swrKeys.monitoringOAuthConnections] = oauthConnections

  return (
    <PageContainer className="p-6">
      <SWRFallback fallback={fallback}>
        <div className="mb-6 flex items-center justify-between gap-4">
          <h1 className="text-3xl font-bold">Settings</h1>
          <Link
            href="/monitoring"
            className="text-sm text-accent underline-offset-4 hover:underline"
          >
            Back to monitoring
          </Link>
        </div>

        <MonitoringSettingsClient />
      </SWRFallback>
    </PageContainer>
  )
}
