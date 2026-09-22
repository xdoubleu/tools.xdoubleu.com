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
  /**
   * Fill the whole viewport on desktop (`lg+`), edge to edge, instead of the
   * centered card the dialog primitive defaults to from `sm` up. The prose
   * column stays capped at its readable width (issue #1867 — feeds only).
   */
  bleedDesktop?: boolean
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
  onContentClick,
  bleedDesktop = false
}: ArticleReaderDialogProps) {
  // The dialog primitive's default is full-bleed below `sm` and a centered
  // card from `sm` up (components/ui/dialog.tsx). `bleedDesktop` opts the
  // feeds reader back into full-bleed — edge to edge, full height — from
  // `lg` up (issue #1867); the prose stays capped via max-w-prose regardless.
  // The default (books reader) keeps the centered-card classes exactly.
  const dialogClassName = bleedDesktop
    ? cn(
        'max-w-none w-full p-4 pt-[calc(1rem+env(safe-area-inset-top))] sm:h-[90vh] sm:p-5 flex flex-col',
        // Neutralize every `sm:` centering rule from the primitive's
        // fullscreenContentClass at `lg`, so the dialog fills the viewport
        // edge to edge instead of collapsing into the centered card.
        'lg:inset-0 lg:h-full lg:w-full lg:max-w-none lg:max-h-full',
        'lg:rounded-none lg:translate-x-0 lg:translate-y-0'
      )
    : 'max-w-2xl lg:max-w-4xl p-4 pt-[calc(1rem+env(safe-area-inset-top))] sm:h-[90vh] sm:p-5 flex flex-col'

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent side="fullscreen" className={dialogClassName}>
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
              // Capped to a readable line length inside the wider `lg`
              // dialog; below `lg` the dialog itself is the cap.
              className={cn(
                'prose prose-sm max-w-none lg:max-w-prose lg:mx-auto text-fg p-1',
                proseClassName
              )}
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
