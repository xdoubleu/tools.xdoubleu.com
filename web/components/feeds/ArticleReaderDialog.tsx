'use client'

import { useEffect, useRef, useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import FeedBookmarkButton from '@/components/feeds/FeedBookmarkButton'
import FeedItemMarkReadButton, {
  type FeedItemMarkReadHandle
} from '@/components/feeds/FeedItemMarkReadButton'
import { useUpdateItem } from '@/hooks/useFeeds'
import type { Item } from '@/lib/gen/feeds/v1/feeds_pb'
import { sanitizeArticleHtml } from '@/lib/sanitizeHtml'

// How close to the bottom (px) counts as "reached the end" (issue #716).
const AUTO_READ_THRESHOLD_PX = 24

// How long to wait after the last scroll event before persisting the
// furthest-reached read-progress percentage (issue #798) — avoids a
// network call on every scroll tick.
const PROGRESS_DEBOUNCE_MS = 1000

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
  const [zoomedSrc, setZoomedSrc] = useState<string | null>(null)
  const markReadRef = useRef<FeedItemMarkReadHandle>(null)
  const updateItem = useUpdateItem()

  // Furthest scroll percentage reached this session, seeded from the item's
  // already-persisted value so re-reaching a prior position never re-sends.
  const maxPctRef = useRef(item.readProgressPct)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    maxPctRef.current = item.readProgressPct
  }, [item.id, item.readProgressPct])

  // Flush on item change or unmount — the debounce alone wouldn't fire if
  // the dialog closes/switches items mid-timer.
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
      if (maxPctRef.current > 0) {
        void updateItem(item.id, { readProgressPct: maxPctRef.current })
      }
    }
  }, [item.id, updateItem])

  const reportProgress = (pct: number) => {
    if (pct <= maxPctRef.current) return
    maxPctRef.current = pct
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      debounceRef.current = null
      void updateItem(item.id, { readProgressPct: maxPctRef.current })
    }, PROGRESS_DEBOUNCE_MS)
  }

  // Auto-mark-read once the reader is scrolled to the end of the content
  // (issue #716); the button's own undo window still lets the user revert.
  const checkAutoRead = (el: HTMLDivElement | null) => {
    if (!el || !html || el.clientHeight === 0) return
    reportProgress(
      Math.min(100, Math.round(((el.scrollTop + el.clientHeight) / el.scrollHeight) * 100))
    )
    if (el.scrollHeight - el.scrollTop - el.clientHeight <= AUTO_READ_THRESHOLD_PX) {
      markReadRef.current?.markRead()
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        side="fullscreen"
        className="max-w-2xl p-4 pt-[calc(1rem+env(safe-area-inset-top))] sm:h-[85vh] sm:p-5 flex flex-col"
      >
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
            <FeedBookmarkButton itemId={item.id} bookmarked={item.bookmarked} />
            <FeedItemMarkReadButton
              ref={markReadRef}
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

        <div
          className="min-w-0 flex-1 overflow-y-auto"
          ref={checkAutoRead}
          onScroll={(e) => checkAutoRead(e.currentTarget)}
        >
          {!html && (
            <p className="text-sm text-muted p-4">
              No in-app content stored for this item.
              {item.sourceUrl && ' Use "View original" above instead.'}
            </p>
          )}

          {html && (
            <div
              className="prose prose-sm max-w-none text-fg p-1 [&_img]:cursor-zoom-in"
              // The HTML originates from third-party RSS feeds — always
              // sanitize before rendering.
              dangerouslySetInnerHTML={{ __html: sanitizeArticleHtml(html) }}
              // Delegated: article images are raw HTML, so there's no per-image
              // React node to attach a handler to (issue #941). preventDefault
              // keeps an image wrapped in a link from navigating away instead.
              onClick={(e) => {
                if (!(e.target instanceof HTMLImageElement)) return
                e.preventDefault()
                setZoomedSrc(e.target.src)
              }}
            />
          )}
        </div>
      </DialogContent>

      <Dialog open={zoomedSrc !== null} onOpenChange={() => setZoomedSrc(null)}>
        {zoomedSrc && (
          <DialogContent
            side="fullscreen"
            className="bg-transparent border-none shadow-none sm:max-w-[90vw]"
          >
            <DialogTitle className="sr-only">Enlarged image</DialogTitle>
            {/* Whole surface closes — no chrome to hunt for on touch. */}
            <DialogClose
              aria-label="Close image"
              className="block h-full w-full cursor-zoom-out p-0"
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={zoomedSrc}
                alt=""
                className="mx-auto max-h-[90vh] w-auto max-w-full object-contain"
              />
            </DialogClose>
          </DialogContent>
        )}
      </Dialog>
    </Dialog>
  )
}
