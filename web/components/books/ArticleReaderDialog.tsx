'use client'

import DOMPurify from 'dompurify'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { useGetBookContent } from '@/hooks/useBooks'

interface ArticleReaderDialogProps {
  bookId: string
  title: string
  sourceUrl?: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

// In-app reader for a library book's stored content (paper/article ingests
// with content_html). RSS feed items have their own reader in
// components/feeds/ArticleReaderDialog.tsx, backed by the standalone feeds
// app (issue #734) — this one stays library/bookId-based.
export default function ArticleReaderDialog({
  bookId,
  title,
  sourceUrl,
  open,
  onOpenChange
}: ArticleReaderDialogProps) {
  const { data, error } = useGetBookContent(open ? bookId : null)
  const html = data?.html ?? ''

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent side="fullscreen" className="p-4 sm:p-5 max-w-2xl sm:h-[85vh] flex flex-col">
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
          <DialogClose
            aria-label="Close reader"
            className="flex h-11 w-11 shrink-0 items-center justify-center text-lg"
          >
            X
          </DialogClose>
        </DialogHeader>

        <div className="min-w-0 flex-1 overflow-y-auto">
          {error && <p className="text-sm text-danger p-4">Failed to load article.</p>}

          {!error && !data && <p className="text-sm text-muted p-4">Loading…</p>}

          {!error && data && !html && (
            <p className="text-sm text-muted p-4">
              No in-app content stored for this item.
              {sourceUrl && ' Use "View original" above instead.'}
            </p>
          )}

          {html && (
            <div
              className="prose prose-sm max-w-none text-foreground p-1"
              // Content originates from ingested third-party HTML — always
              // sanitize before rendering.
              dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(html) }}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
