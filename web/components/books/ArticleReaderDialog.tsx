'use client'

import ArticleReaderDialog from '@/components/ArticleReaderDialog'
import { useGetBookContent } from '@/hooks/useBooks'
import { LoadingState, ErrorState } from '@/components/ui/states'

interface BookArticleReaderDialogProps {
  bookId: string
  title: string
  sourceUrl?: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

// Reader for a library book's stored content_html.
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
      {error && <ErrorState what="article" className="p-4 text-sm" />}

      {!error && !data && <LoadingState className="p-4 text-sm" />}

      {!error && data && !html && (
        <p className="text-sm text-muted p-4">
          No in-app content stored for this item.
          {sourceUrl && ' Use "View original" above instead.'}
        </p>
      )}
    </ArticleReaderDialog>
  )
}
