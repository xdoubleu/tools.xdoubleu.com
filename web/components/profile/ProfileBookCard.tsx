import type { UserBook } from '@/lib/gen/reading/v1/library_pb'
import BookCover from '@/components/reading/BookCover'
import BookProgressBar from '@/components/reading/BookProgressBar'
import { Card } from '@/components/ui/card'
import { statusLabel, displayTags } from '@/lib/reading/bookShelves'

// Read-only book card: no link (book detail pages are owner-only), no
// favourite toggle or status editing.
export default function ProfileBookCard({ userBook }: { userBook: UserBook }) {
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
            <span className="ml-2 text-amber-500" aria-label="Favourite">
              ♥
            </span>
          )}
        </h3>
        <p className="text-sm text-muted truncate">{book.authors.join(', ')}</p>
        <p className="text-sm text-muted">{statusLabel(userBook.status)}</p>
        {userBook.rating > 0 && (
          <p className="text-sm text-amber-500" aria-label={`Rated ${userBook.rating} of 5`}>
            {'★'.repeat(userBook.rating)}
            <span className="text-border">{'★'.repeat(Math.max(0, 5 - userBook.rating))}</span>
          </p>
        )}
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
