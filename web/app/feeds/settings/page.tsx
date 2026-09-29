import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import FeedFilterRulesCard from '@/components/feeds/FeedFilterRulesCard'
import FeedsNotificationSettingsCard from '@/components/feeds/FeedsNotificationSettingsCard'

export default async function FeedsSettingsPage() {
  const [client, feedsClient] = await Promise.all([
    createServerClient(ObservabilityService),
    createServerClient(FeedService)
  ])
  const [notificationSettings, feeds, filterRules] = await Promise.all([
    fetchOrNull(() => client.getNotificationSettings({})),
    fetchOrNull(() => feedsClient.listFeeds({})),
    fetchOrNull(() => feedsClient.listFilterRules({}))
  ])

  const fallback = {
    ...(notificationSettings && {
      [swrKeys.monitoringNotificationSettings]: notificationSettings
    }),
    ...(feeds && { [swrKeys.feeds]: feeds }),
    ...(filterRules && { [swrKeys.feedFilterRules]: filterRules })
  }

  return (
    <PageContainer>
      <SWRFallback fallback={fallback}>
        <PageHeader
          title="Feed Settings"
          breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Settings' }]}
        />

        <div className="space-y-6">
          <FeedFilterRulesCard />
          <FeedsNotificationSettingsCard />
        </div>
      </SWRFallback>
    </PageContainer>
  )
}
