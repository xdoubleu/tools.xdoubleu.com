import Link from 'next/link'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'
import { PageContainer } from '@/components/ui/page-container'
import FeedsNotificationSettingsCard from '@/components/feeds/FeedsNotificationSettingsCard'

export default async function FeedsSettingsPage() {
  const client = await createServerClient(ObservabilityService)
  const notificationSettings = await fetchOrNull(() => client.getNotificationSettings({}))

  return (
    <PageContainer className="p-6">
      <SWRFallback
        fallback={
          notificationSettings
            ? { [swrKeys.monitoringNotificationSettings]: notificationSettings }
            : {}
        }
      >
        <div className="mb-6 flex items-center justify-between gap-4">
          <h1 className="text-3xl font-bold">Feed Settings</h1>
          <Link href="/feeds" className="text-sm text-accent underline-offset-4 hover:underline">
            Back to feeds
          </Link>
        </div>

        <FeedsNotificationSettingsCard />
      </SWRFallback>
    </PageContainer>
  )
}
