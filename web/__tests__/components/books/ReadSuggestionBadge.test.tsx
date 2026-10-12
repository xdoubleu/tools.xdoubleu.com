import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import ReadSuggestionBadge from '@/components/books/ReadSuggestionBadge'
import { ReadSuggestionSchema, UserBookSchema } from '@/lib/gen/books/v1/library_pb'

describe('ReadSuggestionBadge', () => {
  it('shows the suggested device with the figure count as its title', () => {
    const userBook = create(UserBookSchema, {
      formats: ['pdf'],
      readSuggestion: create(ReadSuggestionSchema, { format: 'pdf', figureCount: 30, pages: 100 })
    })
    render(<ReadSuggestionBadge userBook={userBook} />)
    expect(screen.getByText('Best on iPad/laptop (PDF)')).toHaveAttribute(
      'title',
      '30 figures in 100 pages'
    )
  })

  it('renders nothing for an unanalysed book', () => {
    const { container } = render(
      <ReadSuggestionBadge userBook={create(UserBookSchema, { formats: ['epub'] })} />
    )
    expect(container).toBeEmptyDOMElement()
  })
})
