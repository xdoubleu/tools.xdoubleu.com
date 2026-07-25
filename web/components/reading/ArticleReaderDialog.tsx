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
      <DialogContent className="max-w-2xl h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="leading-tight">{title}</DialogTitle>
          {sourceUrl && (
            <a
              href={sourceUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-accent underline-offset-4 hover:underline"
            >
              View original ↗
            </a>
          )}
          <DialogClose aria-label="Close reader">X</DialogClose>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto">
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
