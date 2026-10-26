import FeedReaderClient from '@/components/feeds/FeedReaderClient'
import FeedsHeader from '@/components/feeds/FeedsHeader'
import FeedRuleSuggestionsCard from '@/components/feeds/FeedRuleSuggestionsCard'
import { PageContainer } from '@/components/ui/page-container'

export default function FeedsPage() {
  return (
    <PageContainer>
      <FeedsHeader />

      <FeedRuleSuggestionsCard className="mb-6" />

      <FeedReaderClient />
    </PageContainer>
  )
}
