import { Badge } from '@/components/ui/badge'
import BookCover from '@/components/backlog/BookCover'

// ---------------------------------------------------------------------------
// Duck-typed interfaces — avoids importing branded proto Message types so
// tests can pass plain fixture objects without unsafe type assertions.
// ---------------------------------------------------------------------------

interface DupBook {
  id: string
  title: string
  authors: string[]
  isbn13: string
  isbn10: string
  coverUrl: string
  description: string
  pageCount: number
  externalRefs: Record<string, string>
}

export interface DupUserBook {
  book?: DupBook | null
  status: string
  tags: string[]
  formats: string[]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function isbnDisplay(isbn13: string, isbn10: string): string {
  if (isbn13) return isbn13
  if (isbn10) return isbn10
  return 'No ISBN'
}

function openLibraryId(externalRefs: Record<string, string>): string {
  return externalRefs['openlibrary'] ?? ''
}

// ---------------------------------------------------------------------------
// DuplicateBookSummary
// ---------------------------------------------------------------------------

interface DuplicateBookSummaryProps {
  ub: DupUserBook
}

/**
 * Renders a single UserBook entry inside the "Find duplicates" dialog.
 * Shows every field that the winner-selection algorithm weighs so the admin
 * can see at a glance why one entry was auto-picked and which to keep.
 */
export default function DuplicateBookSummary({ ub }: DuplicateBookSummaryProps) {
  const book = ub.book
  if (!book) return null

  const hasCover = Boolean(book.coverUrl)
  const hasDesc = Boolean(book.description)
  const hasPhysical = ub.tags.includes('own-physical')
  const hasDigital = ub.tags.includes('own-digital')
  const hasPdf = ub.formats.includes('pdf')
  const hasEpub = ub.formats.includes('epub')
  const hasKepub = ub.formats.includes('kepub')

  const isbn = isbnDisplay(book.isbn13, book.isbn10)
  const olId = openLibraryId(book.externalRefs)

  // Build dot-separated metadata tokens.
  const metaTokens: string[] = [isbn]
  if (book.pageCount > 0) metaTokens.push(`${book.pageCount}p`)
  if (hasCover) metaTokens.push('Cover +')
  if (hasDesc) metaTokens.push('Desc +')
  if (olId) metaTokens.push(`OL ${olId}`)

  return (
    <div className="flex items-start gap-3">
      <BookCover coverUrl={book.coverUrl} title={book.title} size="sm" />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium leading-tight">{book.title}</p>
        <p className="text-xs text-muted truncate">{book.authors.join(', ')}</p>

        {/* Metadata line */}
        <p className="text-xs text-subtle mt-0.5">{metaTokens.join(' · ')}</p>

        {/* Badges */}
        <div className="flex flex-wrap gap-1 mt-1">
          <span className="text-xs px-1.5 py-0.5 rounded-full bg-surface text-subtle capitalize">
            {ub.status}
          </span>
          {hasPhysical && <Badge variant="secondary">Physical</Badge>}
          {hasDigital && <Badge variant="secondary">Digital</Badge>}
          {hasPdf && <Badge variant="default">PDF</Badge>}
          {hasEpub && <Badge variant="default">EPUB</Badge>}
          {hasKepub && <Badge variant="default">KEPUB</Badge>}
        </div>
      </div>
    </div>
  )
}
