'use client'

import ArticleReaderDialog from '@/components/ArticleReaderDialog'
import { useGetBookContent } from '@/hooks/useBooks'

interface BookArticleReaderDialogProps {
  bookId: string
  title: string
  sourceUrl?: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

// In-app reader for a library book's stored content (paper/article ingests
// with content_html) — fetches by bookId and renders the shared reader
// scaffold in components/ArticleReaderDialog.tsx.
export default function BookArticleReaderDialog({
  bookId,
  title,
  sourceUrl,
  open,
  onOpenChange
}: BookArticleReaderDialogProps) {
  const { data, error } = useGetBookContent(open ? bookId : null)
  const html = data?.html ?? ''

  return (
    <ArticleReaderDialog
      title={title}
      sourceUrl={sourceUrl}
      open={open}
      onOpenChange={onOpenChange}
      html={html}
    >
      {error && <p className="text-sm text-danger p-4">Failed to load article.</p>}

      {!error && !data && <p className="text-sm text-muted p-4">Loading…</p>}

      {!error && data && !html && (
        <p className="text-sm text-muted p-4">
          No in-app content stored for this item.
          {sourceUrl && ' Use "View original" above instead.'}
        </p>
      )}
    </ArticleReaderDialog>
  )
}
