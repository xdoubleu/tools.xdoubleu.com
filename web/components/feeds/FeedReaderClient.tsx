'use client'

import { useCallback, useMemo, useState } from 'react'
import { useFeeds, useFeedItems, useFetchFeedItemsPage } from '@/hooks/useFeeds'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import ArticleReaderDialog from '@/components/feeds/ArticleReaderDialog'
import FeedBookmarkButton from '@/components/feeds/FeedBookmarkButton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { Select } from '@/components/ui/select'
import { cn } from '@/lib/cn'
import { formatDate } from '@/lib/dates'
import type { Item } from '@/lib/gen/feeds/v1/feeds_pb'

const LAST_VISIT_KEY = 'feeds:lastVisit'

// Items ingested after the visitor's previous visit are "new"; older unread
// items have just been sitting there already seen. Tracked client-side via
// localStorage since the backend has no per-visit read receipt.
function readAndBumpLastVisit(): number {
  if (typeof window === 'undefined') return Date.now()
  const stored = window.localStorage.getItem(LAST_VISIT_KEY)
  window.localStorage.setItem(LAST_VISIT_KEY, String(Date.now()))
  return stored ? Number(stored) : Date.now()
}

export default function FeedReaderClient() {
  const [showRead, setShowRead] = useState(false)
  const unreadOnly = !showRead
  const [bookmarkedOnly, setBookmarkedOnly] = useState(false)
  const [selectedFeedId, setSelectedFeedId] = useState<string | undefined>(undefined)

  const { data: feedsData } = useFeeds()
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

  // Marking read revalidates feedItems immediately, so an unread-only fetch
  // drops the item server-side well before the undo window elapses —
  // pendingRead pins the last-known item so its card (and still-open Undo
  // affordance) stays visible until handleSettled removes the pin.
  const [pendingRead, setPendingRead] = useState<Map<string, Item>>(new Map())

  const items = useMemo(() => {
    if (!unreadOnly || pendingRead.size === 0) return page
    const extra = [...pendingRead.values()].filter((p) => !page.some((i) => i.id === p.id))
    return [...page, ...extra]
  }, [page, pendingRead, unreadOnly])

  const [lastVisit] = useState(readAndBumpLastVisit)

  const feedTitleById = useMemo(() => {
    const map = new Map<string, string>()
    for (const feed of feedsData?.feeds ?? []) {
      map.set(feed.id, feed.title || feed.url)
    }
    return map
  }, [feedsData])

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

  if (isLoading) return <p className="text-muted">Loading…</p>
  if (error) return <p className="text-danger">Failed to load feed items.</p>

  return (
    <div>
      <div className="mb-4 flex justify-end gap-2">
        <Select
          value={selectedFeedId ?? ''}
          onChange={(e) => setSelectedFeedId(e.target.value || undefined)}
          className="w-auto"
        >
          <option value="">All feeds</option>
          {(feedsData?.feeds ?? []).map((feed) => (
            <option key={feed.id} value={feed.id}>
              {feed.title || feed.url}
            </option>
          ))}
        </Select>
        <Button variant="secondary" size="sm" onClick={() => setShowRead((v) => !v)}>
          {showRead ? 'Show unread only' : 'Show read items'}
        </Button>
        <Button
          variant={bookmarkedOnly ? 'default' : 'secondary'}
          size="sm"
          onClick={() => setBookmarkedOnly((v) => !v)}
        >
          {bookmarkedOnly ? 'Show all' : 'Show bookmarked'}
        </Button>
      </div>

      {items.length === 0 ? (
        <p className="py-16 text-center text-sm text-muted">
          {bookmarkedOnly
            ? 'No bookmarked feed items.'
            : showRead
              ? 'No feed items.'
              : 'No unread feed items.'}
        </p>
      ) : (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {items.map((item) => (
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
    </div>
  )
}

interface FeedReaderCardProps {
  item: Item
  feedTitle?: string
  isNew: boolean
  isRead: boolean
  onMarkRead: (item: Item) => void
  onSettled: (itemId: string) => void
}

function FeedReaderCard({
  item,
  feedTitle,
  isNew,
  isRead,
  onMarkRead,
  onSettled
}: FeedReaderCardProps) {
  const [readerOpen, setReaderOpen] = useState(false)
  const noContent = !item.contentHtml
  const handleMarkRead = useCallback(() => onMarkRead(item), [onMarkRead, item])

  // Settling while the reader is open would drop the item from the unread
  // list and unmount this card's dialog out from under the reader (issue
  // #863) — so ignore the undo window elapsing until the reader closes.
  const handleReaderSettled = useCallback(
    (itemId: string) => {
      if (!readerOpen) onSettled(itemId)
    },
    [readerOpen, onSettled]
  )
  // Closing the reader always settles: a read item lingering (pinned to the
  // end of the grid) for the rest of the undo window is issue #913. Undo
  // only ever lives inside the open reader anyway. No-ops when unpinned.
  const handleOpenChange = useCallback(
    (next: boolean) => {
      setReaderOpen(next)
      if (!next) onSettled(item.id)
    },
    [item.id, onSettled]
  )

  return (
    <div
      className={cn(
        'flex flex-col gap-2 rounded-2xl border border-border bg-card p-3 shadow-card',
        isNew && 'border-accent',
        isRead && 'opacity-60'
      )}
    >
      <div className="flex items-start gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-2">
            <Button
              type="button"
              variant="link"
              onClick={() => handleOpenChange(true)}
              className="h-auto justify-start p-0 font-semibold text-sm leading-snug text-fg no-underline hover:text-accent"
            >
              {item.title}
            </Button>
            <div className="flex shrink-0 items-center gap-2">
              {isNew && <Badge variant="default">New</Badge>}
              <FeedBookmarkButton itemId={item.id} bookmarked={item.bookmarked} />
            </div>
          </div>
          {feedTitle && <p className="text-xs text-muted">{feedTitle}</p>}
          <p className="text-xs text-muted">{formatDate(item.publishedAt)}</p>
          {noContent && <p className="text-xs text-subtle">No in-app content</p>}
        </div>
      </div>

      <ArticleReaderDialog
        item={item}
        open={readerOpen}
        onOpenChange={handleOpenChange}
        onMarkRead={handleMarkRead}
        onSettled={handleReaderSettled}
      />
    </div>
  )
}
