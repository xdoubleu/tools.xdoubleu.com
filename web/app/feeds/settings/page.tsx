import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { SettingsSections } from '@/components/ui/settings-section'
import FeedFilterRulesCard from '@/components/feeds/FeedFilterRulesCard'
import FeedsNotificationSettingsCard from '@/components/feeds/FeedsNotificationSettingsCard'

export default function FeedsSettingsPage() {
  return (
    <PageContainer>
      <PageHeader
        title="Feed settings"
        breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Settings' }]}
      />

      <SettingsSections>
        <FeedFilterRulesCard />
        <FeedsNotificationSettingsCard />
      </SettingsSections>
    </PageContainer>
  )
}
