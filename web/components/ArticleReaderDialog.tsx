'use client'

import type { MouseEventHandler, ReactNode, UIEventHandler } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { sanitizeArticleHtml } from '@/lib/sanitizeHtml'
import { cn } from '@/lib/cn'

interface ArticleReaderDialogProps {
  title: string
  sourceUrl?: string
  open: boolean
  onOpenChange: (open: boolean) => void
  actions?: ReactNode
  html?: string
  proseClassName?: string
  /** Loading/error/empty content rendered above the prose. */
  children?: ReactNode
  scrollRef?: (el: HTMLDivElement | null) => void
  onScroll?: UIEventHandler<HTMLDivElement>
  onContentClick?: MouseEventHandler<HTMLDivElement>
  /** Fill the viewport edge to edge on desktop (`lg+`), prose full width. */
  bleedDesktop?: boolean
}

// Clears the notch and home indicator on phones; `dvh` tracks mobile browser chrome.
const safeAreaPadding =
  'p-4 pt-[calc(1rem+env(safe-area-inset-top))] pb-[calc(1rem+env(safe-area-inset-bottom))] sm:h-[90dvh] sm:p-5'

// Full-screen reader scaffold shared by books and feeds: header plus a
// sanitized prose body. Callers fetch their own content.
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
  const dialogClassName = bleedDesktop
    ? cn(
        'max-w-none w-full flex flex-col',
        safeAreaPadding,
        // Undo the primitive's `sm:` centering at `lg`.
        'lg:inset-0 lg:h-full lg:w-full lg:max-w-none lg:max-h-full',
        'lg:rounded-none lg:translate-x-0 lg:translate-y-0'
      )
    : cn('max-w-2xl lg:max-w-4xl flex flex-col', safeAreaPadding)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent side="fullscreen" className={dialogClassName}>
        <DialogHeader className="items-start gap-3">
          <div className="min-w-0 flex-1">
            <DialogTitle className="leading-tight">{title}</DialogTitle>
            {sourceUrl && (
              <Button asChild variant="link" className="mt-1 text-xs">
                <a href={sourceUrl} target="_blank" rel="noopener noreferrer">
                  View original ↗
                </a>
              </Button>
            )}
          </div>
          {actions}
          <DialogClose
            aria-label="Close reader"
            className="m-0 flex h-11 w-11 shrink-0 items-center justify-center text-lg"
          >
            X
          </DialogClose>
        </DialogHeader>

        <div className="min-w-0 flex-1 overflow-y-auto" ref={scrollRef} onScroll={onScroll}>
          {children}

          {html && (
            <div
              className={cn(
                'prose prose-sm max-w-none text-fg p-1',
                !bleedDesktop && 'lg:max-w-prose lg:mx-auto',
                proseClassName
              )}
              dangerouslySetInnerHTML={{ __html: sanitizeArticleHtml(html) }}
              onClick={onContentClick}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
