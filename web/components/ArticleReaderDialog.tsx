'use client'

import type { MouseEventHandler, ReactNode, UIEventHandler } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { sanitizeArticleHtml } from '@/lib/sanitizeHtml'
import { cn } from '@/lib/cn'

interface ArticleReaderDialogProps {
  title: string
  sourceUrl?: string
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Domain-specific header controls, rendered left of the close button. */
  actions?: ReactNode
  /** Raw article HTML; sanitized here before rendering. */
  html?: string
  /** Extra classes for the prose container. */
  proseClassName?: string
  /** Status/placeholder content (loading, error, "no content"), rendered above the prose. */
  children?: ReactNode
  scrollRef?: (el: HTMLDivElement | null) => void
  onScroll?: UIEventHandler<HTMLDivElement>
  onContentClick?: MouseEventHandler<HTMLDivElement>
}

// Full-screen in-app reader scaffold shared by the books and feeds readers:
// header (title, "View original" link, caller-supplied actions, close) plus a
// scrollable, sanitized prose body. It knows nothing about either domain —
// callers fetch their own content and pass it in.
export default function ArticleReaderDialog({
  title,
  sourceUrl,
  open,
  onOpenChange,
  actions,
  html,
  proseClassName = '',
  children,
  scrollRef,
  onScroll,
  onContentClick
}: ArticleReaderDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        side="fullscreen"
        className="max-w-2xl p-4 pt-[calc(1rem+env(safe-area-inset-top))] sm:h-[85vh] sm:p-5 flex flex-col"
      >
        <DialogHeader className="items-start gap-3">
          <div className="min-w-0 flex-1">
            <DialogTitle className="leading-tight">{title}</DialogTitle>
            {sourceUrl && (
              <a
                href={sourceUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-1 inline-block py-1 text-xs text-accent underline-offset-4 hover:underline"
              >
                View original ↗
              </a>
            )}
          </div>
          {actions}
          <DialogClose
            aria-label="Close reader"
            className="flex h-11 w-11 shrink-0 items-center justify-center text-lg"
          >
            X
          </DialogClose>
        </DialogHeader>

        <div className="min-w-0 flex-1 overflow-y-auto" ref={scrollRef} onScroll={onScroll}>
          {children}

          {html && (
            <div
              className={cn('prose prose-sm max-w-none text-fg p-1', proseClassName)}
              // Article bodies originate from ingested third-party HTML —
              // always sanitize before rendering.
              dangerouslySetInnerHTML={{ __html: sanitizeArticleHtml(html) }}
              onClick={onContentClick}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
