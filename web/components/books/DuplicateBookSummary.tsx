import { Badge } from '@/components/ui/badge'
import BookCover from '@/components/books/BookCover'

// Duck-typed so tests can pass plain fixtures instead of proto Messages.

interface DupBook {
  id: string
  title: string
  authors: string[]
  isbn13: string
  coverUrl: string
  description: string
  pageCount: number
}

export interface DupUserBook {
  book?: DupBook | null
  status: string
  tags: string[]
  formats: string[]
}

function isbnDisplay(isbn13: string): string {
  if (isbn13) return `ISBN ${isbn13}`
  return 'No ISBN'
}

interface DuplicateBookSummaryProps {
  ub: DupUserBook
}

// Metadata quality helpers (mirror metadataCompleteness in book_matching.go).

interface MetadataField {
  label: string
  present: boolean
}

function metadataFields(book: DupBook): MetadataField[] {
  return [
    { label: 'Authors', present: book.authors.length > 0 },
    { label: 'ISBN-13', present: Boolean(book.isbn13) },
    { label: 'Cover', present: Boolean(book.coverUrl) },
    { label: 'Description', present: Boolean(book.description) },
    { label: 'Page count', present: book.pageCount > 0 }
  ]
}

export default function DuplicateBookSummary({ ub }: DuplicateBookSummaryProps) {
  const book = ub.book
  if (!book) return null

  const hasPhysical = ub.tags.includes('own-physical')
  const hasDigital = ub.tags.includes('own-digital')
  const hasPdf = ub.formats.includes('pdf')
  const hasEpub = ub.formats.includes('epub')
  const hasKepub = ub.formats.includes('kepub')

  const isbn = isbnDisplay(book.isbn13)
  const fields = metadataFields(book)
  const score = fields.filter((f) => f.present).length

  const metaTokens: string[] = [isbn]
  if (book.pageCount > 0) metaTokens.push(`${book.pageCount}p`)

  return (
    <div className="flex items-start gap-3">
      <BookCover coverUrl={book.coverUrl} title={book.title} size="sm" />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium leading-tight">{book.title}</p>
        <p className="text-xs text-muted truncate">{book.authors.join(', ')}</p>

        <p className="text-xs text-subtle mt-0.5">{metaTokens.join(' · ')}</p>

        <div className="flex flex-wrap gap-1 mt-1">
          <span className="text-xs px-1.5 py-0.5 rounded-full bg-surface text-subtle">
            Metadata {score}/5
          </span>
          {fields.map((f) => (
            <Badge key={f.label} variant={f.present ? 'default' : 'secondary'}>
              {f.present ? f.label : `No ${f.label.toLowerCase()}`}
            </Badge>
          ))}
        </div>

        <div className="flex flex-wrap gap-1 mt-1">
          <span className="text-xs px-1.5 py-0.5 rounded-full bg-surface text-subtle capitalize">
            {ub.status}
          </span>
          {hasPhysical && <Badge variant="secondary">Physical</Badge>}
          {hasDigital && <Badge variant="default">Digital</Badge>}
          {hasPdf && <Badge variant="default">PDF</Badge>}
          {hasEpub && <Badge variant="default">EPUB</Badge>}
          {hasKepub && <Badge variant="default">KEPUB</Badge>}
        </div>
      </div>
    </div>
  )
}
