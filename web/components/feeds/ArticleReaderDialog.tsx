'use client'

import { useEffect, useRef, useState } from 'react'
import ArticleReaderDialog from '@/components/ArticleReaderDialog'
import { Dialog, DialogContent, DialogTitle, DialogClose } from '@/components/ui/dialog'
import { ErrorState, LoadingState } from '@/components/ui/states'
import FeedBookmarkButton from '@/components/feeds/FeedBookmarkButton'
import FeedItemMarkReadButton, {
  type FeedItemMarkReadHandle
} from '@/components/feeds/FeedItemMarkReadButton'
import { useFeedItem, useUpdateItem } from '@/hooks/useFeeds'
import { ConnectError, Code } from '@connectrpc/connect'
import type { Item } from '@/lib/gen/feeds/v1/feeds_pb'

// Distance from the bottom (px) that counts as reaching the end.
const AUTO_READ_THRESHOLD_PX = 24

// Debounce before persisting the furthest read-progress percentage.
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

// List responses carry only hasContent, so the reader fetches the body on open.
export default function FeedArticleReaderDialog({
  item,
  open,
  onOpenChange,
  onMarkRead,
  onSettled
}: FeedArticleReaderDialogProps) {
  const { data: itemData, isLoading, error } = useFeedItem(open && item.hasContent ? item.id : null)
  const html = itemData?.item?.contentHtml ?? ''
  // The item may have been deleted since the list was fetched.
  const notFound = error instanceof ConnectError && error.code === Code.NotFound
  const [zoomedSrc, setZoomedSrc] = useState<string | null>(null)
  const markReadRef = useRef<FeedItemMarkReadHandle>(null)
  const updateItem = useUpdateItem()

  // Seeded from the persisted value so a prior position is never re-sent.
  const maxPctRef = useRef(item.readProgressPct)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    maxPctRef.current = item.readProgressPct
  }, [item.id, item.readProgressPct])

  // Flush on item change or unmount; the debounce wouldn't fire.
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

  // Auto-mark-read at the end of the content (undoable). Only when it's
  // actually scrollable, so a short article isn't marked read on open.
  const checkAutoRead = (el: HTMLDivElement | null) => {
    if (!el || !html || el.clientHeight === 0) return
    reportProgress(
      Math.min(100, Math.round(((el.scrollTop + el.clientHeight) / el.scrollHeight) * 100))
    )
    if (
      el.scrollHeight > el.clientHeight &&
      el.scrollHeight - el.scrollTop - el.clientHeight <= AUTO_READ_THRESHOLD_PX
    ) {
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
        bleedDesktop
        proseClassName="[&_img]:cursor-zoom-in"
        scrollRef={checkAutoRead}
        onScroll={(e) => checkAutoRead(e.currentTarget)}
        // Delegated: article images are raw HTML. preventDefault stops a linked
        // image from navigating.
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

        {item.hasContent && notFound && (
          <p className="text-danger p-4">
            This article is no longer available — it may have been deleted.
          </p>
        )}

        {item.hasContent && isLoading && !error && <LoadingState className="p-4" />}
        {item.hasContent && error && !notFound && <ErrorState what="the article" className="p-4" />}
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
                className="mx-auto max-h-[90dvh] w-auto max-w-full object-contain"
              />
            </DialogClose>
          </DialogContent>
        )}
      </Dialog>
    </>
  )
}
