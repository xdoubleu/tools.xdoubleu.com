import UnhealthyFeeds from '@/components/feeds/UnhealthyFeeds'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

// Admin-only RPC: non-admins get null and nothing renders.
export default function FeedHealthPage() {
  return (
    <PageContainer>
      <PageHeader
        title="Feed Health"
        breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Health' }]}
      />

      <UnhealthyFeeds />
    </PageContainer>
  )
}
