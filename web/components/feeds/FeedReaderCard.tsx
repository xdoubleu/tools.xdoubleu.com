import { useCallback, useState } from 'react'
import ArticleReaderDialog from '@/components/feeds/ArticleReaderDialog'
import FeedBookmarkButton from '@/components/feeds/FeedBookmarkButton'
import FeedItemCategories from '@/components/feeds/FeedItemCategories'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { cn } from '@/lib/cn'
import { formatDate } from '@/lib/dates'
import { expiresLabel } from '@/lib/feeds/expiry'
import type { Item } from '@/lib/gen/feeds/v1/feeds_pb'

interface FeedReaderCardProps {
  item: Item
  feedTitle?: string
  isNew: boolean
  isRead: boolean
  onMarkRead: (item: Item) => void
  onSettled: (itemId: string) => void
}

export default function FeedReaderCard({
  item,
  feedTitle,
  isNew,
  isRead,
  onMarkRead,
  onSettled
}: FeedReaderCardProps) {
  const [readerOpen, setReaderOpen] = useState(false)
  const noContent = !item.hasContent
  const expires = isRead ? null : expiresLabel(item.expiresAt)
  const handleMarkRead = useCallback(() => onMarkRead(item), [onMarkRead, item])

  // Settling while the reader is open would unmount its dialog, so wait.
  const handleReaderSettled = useCallback(
    (itemId: string) => {
      if (!readerOpen) onSettled(itemId)
    },
    [readerOpen, onSettled]
  )
  // Closing the reader always settles (Undo only lives in the reader).
  const handleOpenChange = useCallback(
    (next: boolean) => {
      setReaderOpen(next)
      if (!next) onSettled(item.id)
    },
    [item.id, onSettled]
  )

  return (
    <Card
      padding="sm"
      className={cn('flex flex-col gap-2', isNew && 'border-accent', isRead && 'opacity-60')}
    >
      <div className="flex items-start gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-2">
            <Button
              type="button"
              variant="link"
              onClick={() => handleOpenChange(true)}
              // A wrapped title still inherits the button's text-align: center.
              className="h-auto min-w-0 justify-start p-0 wrap-anywhere text-left font-semibold text-sm leading-snug text-fg no-underline hover:text-accent"
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
          {expires && (
            // eslint-disable-next-line ui/use-primitives -- expiry countdown, not an error
            <p className="text-xs font-medium text-danger">{expires}</p>
          )}
          {noContent && (
            // eslint-disable-next-line ui/use-primitives -- item metadata, not an empty list
            <p className="text-xs text-subtle">No in-app content</p>
          )}
          <FeedItemCategories categories={item.categories} className="mt-1" />
        </div>
      </div>

      <ArticleReaderDialog
        item={item}
        open={readerOpen}
        onOpenChange={handleOpenChange}
        onMarkRead={handleMarkRead}
        onSettled={handleReaderSettled}
      />
    </Card>
  )
}
