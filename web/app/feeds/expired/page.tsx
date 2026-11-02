import { useSearchParams } from '@/hooks/navigation'
import FeedExpiredItemsClient from '@/components/feeds/FeedExpiredItemsClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function FeedExpiredPage() {
  const feedId = useSearchParams().get('feed') || undefined

  return (
    <PageContainer>
      <PageHeader
        title="Expired items"
        description="Unread items that outlived their feed's lifetime. A restored item returns to the inbox as unread and never expires again."
        breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Expired' }]}
      />

      <FeedExpiredItemsClient initialFeedId={feedId} />
    </PageContainer>
  )
}
