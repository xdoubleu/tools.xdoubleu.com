import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import FeedsNotificationSettingsCard from '@/components/feeds/FeedsNotificationSettingsCard'

export default async function FeedsSettingsPage() {
  const client = await createServerClient(ObservabilityService)
  const notificationSettings = await fetchOrNull(() => client.getNotificationSettings({}))

  return (
    <PageContainer>
      <SWRFallback
        fallback={
          notificationSettings
            ? { [swrKeys.monitoringNotificationSettings]: notificationSettings }
            : {}
        }
      >
        <PageHeader
          title="Feed Settings"
          breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Settings' }]}
        />

        <FeedsNotificationSettingsCard />
      </SWRFallback>
    </PageContainer>
  )
}
