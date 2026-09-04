'use client'

import { useEffect, useRef, useState } from 'react'
import ArticleReaderDialog from '@/components/ArticleReaderDialog'
import { Dialog, DialogContent, DialogTitle, DialogClose } from '@/components/ui/dialog'
import FeedBookmarkButton from '@/components/feeds/FeedBookmarkButton'
import FeedItemMarkReadButton, {
  type FeedItemMarkReadHandle
} from '@/components/feeds/FeedItemMarkReadButton'
import { useFeedItem, useUpdateItem } from '@/hooks/useFeeds'
import type { Item } from '@/lib/gen/feeds/v1/feeds_pb'

// How close to the bottom (px) counts as "reached the end" (issue #716).
const AUTO_READ_THRESHOLD_PX = 24

// How long to wait after the last scroll event before persisting the
// furthest-reached read-progress percentage (issue #798) — avoids a
// network call on every scroll tick.
const PROGRESS_DEBOUNCE_MS = 1000

interface FeedArticleReaderDialogProps {
  item: Item
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called synchronously when the user clicks Mark read. */
  onMarkRead: (itemId: string) => void
  /** Called once the mark-read undo window elapses. */
  onSettled: (itemId: string) => void
}

// List responses carry only item.hasContent, never the body (issue #1027),
// so the reader fetches the article itself once the dialog opens — which is
// also the only moment a body is actually needed. The dialog scaffold and
// prose rendering come from the shared components/ArticleReaderDialog.tsx.
export default function FeedArticleReaderDialog({
  item,
  open,
  onOpenChange,
  onMarkRead,
  onSettled
}: FeedArticleReaderDialogProps) {
  const { data: itemData, isLoading } = useFeedItem(open && item.hasContent ? item.id : null)
  const html = itemData?.item?.contentHtml ?? ''
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
    <>
      <ArticleReaderDialog
        title={item.title}
        sourceUrl={item.sourceUrl}
        open={open}
        onOpenChange={onOpenChange}
        html={html}
        proseClassName="[&_img]:cursor-zoom-in"
        scrollRef={checkAutoRead}
        onScroll={(e) => checkAutoRead(e.currentTarget)}
        // Delegated: article images are raw HTML, so there's no per-image
        // React node to attach a handler to (issue #941). preventDefault
        // keeps an image wrapped in a link from navigating away instead.
        onContentClick={(e) => {
          if (!(e.target instanceof HTMLImageElement)) return
          e.preventDefault()
          setZoomedSrc(e.target.src)
        }}
        actions={
          <div className="flex shrink-0 items-center gap-2">
            <FeedBookmarkButton itemId={item.id} bookmarked={item.bookmarked} />
            <FeedItemMarkReadButton
              ref={markReadRef}
              itemId={item.id}
              onMarkRead={onMarkRead}
              onSettled={onSettled}
            />
          </div>
        }
      >
        {!item.hasContent && (
          <p className="text-sm text-muted p-4">
            No in-app content stored for this item.
            {item.sourceUrl && ' Use "View original" above instead.'}
          </p>
        )}

        {item.hasContent && isLoading && <p className="text-muted p-4">Loading…</p>}
      </ArticleReaderDialog>

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
    </>
  )
}
