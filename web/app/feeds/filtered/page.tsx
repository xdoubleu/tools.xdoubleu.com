import { useSearchParams } from '@/hooks/navigation'
import FeedFilteredItemsClient from '@/components/feeds/FeedFilteredItemsClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function FeedFilteredPage() {
  const feedId = useSearchParams().get('feed') || undefined

  return (
    <PageContainer>
      <PageHeader
        title="Filtered items"
        description="Items your filter rules hid. A restored item returns to the inbox as unread, and no rule hides it again."
        breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Filtered' }]}
      />

      <FeedFilteredItemsClient initialFeedId={feedId} />
    </PageContainer>
  )
}
