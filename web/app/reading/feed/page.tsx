import { Suspense } from 'react'
import FeedReaderClient from '@/app/reading/feed/FeedReaderClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/reading/v1/library_pb'
import { FeedService } from '@/lib/gen/reading/v1/feeds_pb'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default async function FeedReaderPage() {
  const libraryClient = await createServerClient(LibraryService)
  const library = await fetchOrNull(() => libraryClient.getLibrary({}))

  const feedsClient = await createServerClient(FeedService)
  const feedItems = await fetchOrNull(() => feedsClient.listFeedItems({}))

  return (
    <PageContainer className="p-6" size="narrow">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Reading', href: '/reading' }, { label: 'Feed' }]}
      />

      <h1 className="mb-6 text-3xl font-bold">Feed</h1>

      <SWRFallback
        fallback={{
          ...(library ? { [swrKeys.books]: library } : {}),
          ...(feedItems ? { [swrKeys.bookFeedItems]: feedItems } : {})
        }}
      >
        <Suspense fallback={<p className="text-muted">Loading…</p>}>
          <FeedReaderClient />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
