import FeedStatsClient from '@/components/feeds/FeedStatsClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function FeedStatsPage() {
  return (
    <PageContainer>
      <PageHeader
        title="Feed Stats"
        breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Stats' }]}
      />

      <FeedStatsClient />
    </PageContainer>
  )
}
