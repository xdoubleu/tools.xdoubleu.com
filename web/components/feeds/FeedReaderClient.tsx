'use client'

import { useCallback, useMemo, useState } from 'react'
import { useFeeds, useFeedItems } from '@/hooks/useFeeds'
import ArticleReaderDialog from '@/components/feeds/ArticleReaderDialog'
import FeedFavouriteButton from '@/components/feeds/FeedFavouriteButton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
  const { data: feedsData } = useFeeds()
  const { data: itemsData, error, isLoading } = useFeedItems()
  const [settled, setSettled] = useState<Set<string>>(new Set())
  const [lastVisit] = useState(readAndBumpLastVisit)

  const feedTitleById = useMemo(() => {
    const map = new Map<string, string>()
    for (const feed of feedsData?.feeds ?? []) {
      map.set(feed.id, feed.title || feed.url)
    }
    return map
  }, [feedsData])

  const unread = useMemo(() => {
    const items = itemsData?.items ?? []
    return items.filter((item) => !item.readAt && !item.dismissed && !settled.has(item.id))
  }, [itemsData, settled])

  const handleSettled = useCallback((itemId: string) => {
    setSettled((prev) => new Set(prev).add(itemId))
  }, [])

  if (isLoading) return <p className="text-muted">Loading…</p>
  if (error) return <p className="text-danger">Failed to load feed items.</p>

  if (unread.length === 0) {
    return <p className="py-16 text-center text-sm text-muted">No unread feed items.</p>
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      {unread.map((item) => (
        <FeedReaderCard
          key={item.id}
          item={item}
          feedTitle={feedTitleById.get(item.feedId)}
          isNew={new Date(item.createdAt).getTime() > lastVisit}
          onSettled={handleSettled}
        />
      ))}
    </div>
  )
}

interface FeedReaderCardProps {
  item: Item
  feedTitle?: string
  isNew: boolean
  onSettled: (itemId: string) => void
}

function FeedReaderCard({ item, feedTitle, isNew, onSettled }: FeedReaderCardProps) {
  const [readerOpen, setReaderOpen] = useState(false)
  const noContent = !item.contentHtml

  return (
    <div
      className={cn(
        'flex flex-col gap-2 rounded-2xl border border-border bg-card p-3 shadow-card',
        isNew && 'border-accent'
      )}
    >
      <div className="flex items-start gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-2">
            <Button
              type="button"
              variant="link"
              onClick={() => setReaderOpen(true)}
              className="h-auto p-0 font-semibold text-sm leading-snug text-fg no-underline hover:text-accent"
            >
              {item.title}
            </Button>
            <div className="flex shrink-0 items-center gap-2">
              {isNew && <Badge variant="default">New</Badge>}
              <FeedFavouriteButton itemId={item.id} favourite={item.favourite} />
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
        onOpenChange={setReaderOpen}
        onSettled={onSettled}
      />
    </div>
  )
}
