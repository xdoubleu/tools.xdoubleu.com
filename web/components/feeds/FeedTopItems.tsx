import { useCallback, useMemo, useState } from 'react'
import { useFeeds } from '@/hooks/useFeeds'
import { useTopFeedItems } from '@/hooks/useFeedExpiry'
import FeedReaderCard from '@/components/feeds/FeedReaderCard'
import { ErrorState, LoadingState } from '@/components/ui/states'
import type { Item } from '@/lib/gen/feeds/v1/feeds_pb'

// Today's ten best items, ranked from your own reading history. A finite
// list: once everything is read it says so instead of loading more.
export default function FeedTopItems() {
  const { data: feedsData } = useFeeds()
  const { data, error, isLoading } = useTopFeedItems(true)
  const [pendingRead, setPendingRead] = useState<Set<string>>(new Set())

  const feedTitleById = useMemo(
    () => new Map((feedsData?.feeds ?? []).map((f) => [f.id, f.title || f.url])),
    [feedsData]
  )
  const handleMarkRead = useCallback((item: Item) => {
    setPendingRead((prev) => new Set(prev).add(item.id))
  }, [])
  const handleSettled = useCallback(() => undefined, [])

  if (isLoading) return <LoadingState placement="page" />
  if (error) return <ErrorState placement="page" what="today's top items" />

  const items = data?.items ?? []
  const done = items.every((item) => Boolean(item.readAt) || pendingRead.has(item.id))

  return (
    <div>
      {items.length > 0 && (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {items.map((item, index) => (
            <div key={item.id} className="flex flex-col gap-1">
              <span className="text-xs font-semibold text-subtle">#{index + 1}</span>
              <FeedReaderCard
                item={item}
                feedTitle={feedTitleById.get(item.feedId)}
                isNew={false}
                isRead={Boolean(item.readAt) || pendingRead.has(item.id)}
                onMarkRead={handleMarkRead}
                onSettled={handleSettled}
              />
            </div>
          ))}
        </div>
      )}
      {done && (
        <p className="py-12 text-center text-sm text-muted">
          {items.length === 0 ? 'Nothing unread today.' : 'You’re done for today.'}
        </p>
      )}
    </div>
  )
}
