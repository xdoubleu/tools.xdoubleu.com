import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookCover from '@/components/books/BookCover'
import BookRatingStars from '@/components/books/BookRatingStars'
import BookProgressBar from '@/components/books/BookProgressBar'
import { Card } from '@/components/ui/card'
import { statusLabel, displayTags } from '@/lib/books/bookShelves'

// No link: book detail pages are owner-only.
export default function DashboardBookCard({ userBook }: { userBook: UserBook }) {
  const book = userBook.book
  if (!book) return null
  const tags = displayTags(userBook.tags)
  return (
    <Card className="flex gap-3 p-4">
      <BookCover coverUrl={book.coverUrl} title={book.title} size="md" />
      <div className="min-w-0 flex-1">
        <h3 className="font-semibold truncate">
          {book.title}
          {userBook.tags.includes('favourite') && (
            <span className="ml-2 text-star" aria-label="Favourite">
              ♥
            </span>
          )}
        </h3>
        <p className="text-sm text-muted truncate">{book.authors.join(', ')}</p>
        <p className="text-sm text-muted">{statusLabel(userBook.status)}</p>
        {userBook.rating > 0 && <BookRatingStars userBook={userBook} readOnly />}
        {userBook.status === 'currently-reading' && (
          <div className="mt-2">
            <BookProgressBar userBook={userBook} />
          </div>
        )}
        {tags.length > 0 && <p className="text-xs text-muted truncate mt-1">{tags.join(', ')}</p>}
      </div>
    </Card>
  )
}
