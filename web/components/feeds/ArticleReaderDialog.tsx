'use client'

import DOMPurify from 'dompurify'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import FeedFavouriteButton from '@/components/feeds/FeedFavouriteButton'
import FeedItemMarkReadButton from '@/components/feeds/FeedItemMarkReadButton'
import type { Item } from '@/lib/gen/feeds/v1/feeds_pb'

interface ArticleReaderDialogProps {
  item: Item
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called synchronously when the user clicks Mark read. */
  onMarkRead: (itemId: string) => void
  /** Called once the mark-read undo window elapses. */
  onSettled: (itemId: string) => void
}

// Item now carries its own content_html (no more library lookup) — the
// reader renders it directly, no separate content fetch needed.
export default function ArticleReaderDialog({
  item,
  open,
  onOpenChange,
  onMarkRead,
  onSettled
}: ArticleReaderDialogProps) {
  const html = item.contentHtml

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent side="fullscreen" className="p-4 sm:p-5 max-w-2xl sm:h-[85vh] flex flex-col">
        <DialogHeader className="items-start gap-3">
          <div className="min-w-0 flex-1">
            <DialogTitle className="leading-tight">{item.title}</DialogTitle>
            {item.sourceUrl && (
              <a
                href={item.sourceUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-1 inline-block py-1 text-xs text-accent underline-offset-4 hover:underline"
              >
                View original ↗
              </a>
            )}
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <FeedFavouriteButton itemId={item.id} favourite={item.favourite} />
            <FeedItemMarkReadButton
              itemId={item.id}
              onMarkRead={onMarkRead}
              onSettled={onSettled}
            />
          </div>
          <DialogClose
            aria-label="Close reader"
            className="flex h-11 w-11 shrink-0 items-center justify-center text-lg"
          >
            X
          </DialogClose>
        </DialogHeader>

        <div className="min-w-0 flex-1 overflow-y-auto">
          {!html && (
            <p className="text-sm text-muted p-4">
              No in-app content stored for this item.
              {item.sourceUrl && ' Use "View original" above instead.'}
            </p>
          )}

          {html && (
            <div
              className="prose prose-sm max-w-none text-foreground p-1"
              // The HTML originates from third-party RSS feeds — always
              // sanitize before rendering.
              dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(html) }}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
