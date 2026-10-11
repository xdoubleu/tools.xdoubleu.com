import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { Badge } from '@/components/ui/badge'
import { readSuggestionText } from '@/lib/books/readSuggestion'

/** Which device (and format) suits the book, from its figure density. */
export default function ReadSuggestionBadge({ userBook }: { userBook: UserBook }) {
  const text = readSuggestionText(userBook.readSuggestion, userBook.formats)
  if (!text) return null
  return (
    <Badge variant="secondary" title={text.detail}>
      {text.label}
    </Badge>
  )
}
