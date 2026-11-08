import { useCallback, useMemo, useState } from 'react'
import {
  useFeeds,
  useFeedItems,
  useFetchFeedItemsPage,
  usePrefetchFeedBodies
} from '@/hooks/useFeeds'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import { useLastChanceFeedItems } from '@/hooks/useFeedExpiry'
import FeedLastChanceStrip from '@/components/feeds/FeedLastChanceStrip'
import FeedReaderCard from '@/components/feeds/FeedReaderCard'
import FeedTopItems from '@/components/feeds/FeedTopItems'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { Select } from '@/components/ui/select'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { Toolbar } from '@/components/ui/toolbar'
import type { Feed, Item } from '@/lib/gen/feeds/v1/feeds_pb'

const LAST_VISIT_KEY = 'feeds:lastVisit'

// Items newer than the previous visit are "new"; tracked in localStorage.
function readAndBumpLastVisit(): number {
  if (typeof window === 'undefined') return Date.now()
  const stored = window.localStorage.getItem(LAST_VISIT_KEY)
  window.localStorage.setItem(LAST_VISIT_KEY, String(Date.now()))
  return stored ? Number(stored) : Date.now()
}

type View = 'unread' | 'all' | 'bookmarked' | 'top'

const VIEWS: { value: View; label: string }[] = [
  { value: 'unread', label: 'Unread' },
  { value: 'all', label: 'All' },
  { value: 'bookmarked', label: 'Bookmarked' },
  { value: 'top', label: 'Today’s top 10' }
]

// The inbox (unread, all or bookmarked), or today's ranked top ten.
export default function FeedReaderClient() {
  const [view, setView] = useState<View>('unread')
  const [selectedFeedId, setSelectedFeedId] = useState<string | undefined>(undefined)
  const { data: feedsData } = useFeeds()

  return (
    <>
      <Toolbar aria-label="Filter feed items">
        <SegmentedTabs aria-label="Show" value={view} options={VIEWS} onChange={setView} />
        {view !== 'top' && (
          <Select
            value={selectedFeedId ?? ''}
            onChange={(e) => setSelectedFeedId(e.target.value || undefined)}
            aria-label="Filter by feed"
            className="w-full cursor-pointer sm:w-auto"
          >
            <option value="">All feeds</option>
            {(feedsData?.feeds ?? []).map((feed) => (
              <option key={feed.id} value={feed.id}>
                {feed.title || feed.url}
              </option>
            ))}
          </Select>
        )}
      </Toolbar>
      {view === 'top' ? (
        <FeedTopItems />
      ) : (
        <FeedInbox
          bookmarkedOnly={view === 'bookmarked'}
          showRead={view === 'all'}
          selectedFeedId={selectedFeedId}
          feeds={feedsData?.feeds ?? []}
        />
      )}
    </>
  )
}

interface FeedInboxProps {
  bookmarkedOnly: boolean
  showRead: boolean
  selectedFeedId?: string
  feeds: Feed[]
}

function FeedInbox({ bookmarkedOnly, showRead, selectedFeedId, feeds }: FeedInboxProps) {
  usePrefetchFeedBodies()
  // Bookmarks are a keep-list, so that view ignores the unread filter.
  const unreadOnly = !showRead && !bookmarkedOnly

  const {
    data: itemsData,
    error,
    isLoading
  } = useFeedItems(unreadOnly, selectedFeedId, bookmarkedOnly)
  const fetchPage = useFetchFeedItemsPage(unreadOnly, selectedFeedId, bookmarkedOnly)
  const initialPage = useMemo(
    () => ({ items: itemsData?.items ?? [], hasMore: itemsData?.hasMore ?? false }),
    [itemsData]
  )
  const {
    items: page,
    hasMore,
    loading: loadingMore,
    loadMore
  } = usePaginatedList(initialPage, fetchPage, (a, b) => a.id === b.id)

  // Revalidation drops a read item before the undo window ends; pendingRead
  // keeps its card (and Undo) visible until handleSettled.
  const [pendingRead, setPendingRead] = useState<Map<string, Item>>(new Map())

  const items = useMemo(() => {
    if (!unreadOnly || pendingRead.size === 0) return page
    const extra = [...pendingRead.values()].filter((p) => !page.some((i) => i.id === p.id))
    return [...page, ...extra]
  }, [page, pendingRead, unreadOnly])

  const [lastVisit] = useState(readAndBumpLastVisit)

  // Only the plain unread inbox shows the strip; its items leave the grid.
  const { data: lastChanceData } = useLastChanceFeedItems(unreadOnly)
  const lastChance = useMemo(
    () =>
      (lastChanceData?.items ?? []).filter(
        (item) => !selectedFeedId || item.feedId === selectedFeedId
      ),
    [lastChanceData, selectedFeedId]
  )
  const gridItems = useMemo(() => {
    if (lastChance.length === 0) return items
    const stripIds = new Set(lastChance.map((item) => item.id))
    return items.filter((item) => !stripIds.has(item.id))
  }, [items, lastChance])

  const feedTitleById = useMemo(() => {
    const map = new Map<string, string>()
    for (const feed of feeds) {
      map.set(feed.id, feed.title || feed.url)
    }
    return map
  }, [feeds])

  const handleMarkRead = useCallback((item: Item) => {
    setPendingRead((prev) => new Map(prev).set(item.id, item))
  }, [])

  const handleSettled = useCallback((itemId: string) => {
    setPendingRead((prev) => {
      if (!prev.has(itemId)) return prev
      const next = new Map(prev)
      next.delete(itemId)
      return next
    })
  }, [])

  if (isLoading) return <LoadingState placement="page" />
  if (error) return <ErrorState what="feed items" placement="page" />

  return (
    <>
      {unreadOnly && (
        <FeedLastChanceStrip
          items={lastChance}
          feedTitleById={feedTitleById}
          lastVisit={lastVisit}
          pendingRead={pendingRead}
          onMarkRead={handleMarkRead}
          onSettled={handleSettled}
        />
      )}

      {gridItems.length === 0 && lastChance.length === 0 ? (
        <EmptyState placement="page">
          {bookmarkedOnly
            ? 'No bookmarked feed items.'
            : showRead
              ? 'No feed items.'
              : 'No unread feed items.'}
        </EmptyState>
      ) : (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {gridItems.map((item) => (
              <FeedReaderCard
                key={item.id}
                item={item}
                feedTitle={feedTitleById.get(item.feedId)}
                isNew={new Date(item.createdAt).getTime() > lastVisit}
                isRead={Boolean(item.readAt) || pendingRead.has(item.id)}
                onMarkRead={handleMarkRead}
                onSettled={handleSettled}
              />
            ))}
          </div>
          {hasMore && <LoadMoreButton onClick={loadMore} loading={loadingMore} />}
        </>
      )}
    </>
  )
}
