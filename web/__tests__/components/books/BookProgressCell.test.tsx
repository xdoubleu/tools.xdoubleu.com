import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'

jest.mock('@/components/books/BookProgressBar', () => {
  return function MockProgressBar() {
    return <div role="progressbar" data-testid="progress-bar" />
  }
})

jest.mock('@/components/books/BookProgressForm', () => {
  return function MockProgressForm() {
    return <div data-testid="progress-form" />
  }
})

import BookProgressCell from '@/components/books/BookProgressCell'

function makeBook(status: string, withBook = true) {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status,
    formats: [],
    book: withBook
      ? create(BookSchema, { title: 'Test Book', authors: ['Author'], pageCount: 200 })
      : undefined
  })
}

describe('BookProgressCell', () => {
  it('renders nothing for a book that is not currently reading', () => {
    const { container } = render(<BookProgressCell userBook={makeBook('to-read')} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders the progress bar trigger for a currently-reading book', () => {
    render(<BookProgressCell userBook={makeBook('currently-reading')} />)
    expect(screen.getByTestId('progress-bar')).toBeInTheDocument()
    expect(screen.queryByTestId('progress-form')).not.toBeInTheDocument()
  })

  it('opens the edit form in a popover when the trigger is clicked', () => {
    render(<BookProgressCell userBook={makeBook('currently-reading')} />)
    fireEvent.click(screen.getByLabelText('Edit reading progress for Test Book'))
    expect(screen.getByTestId('progress-form')).toBeInTheDocument()
  })

  it('falls back to a generic label when the book has no title', () => {
    render(<BookProgressCell userBook={makeBook('currently-reading', false)} />)
    expect(screen.getByLabelText('Edit reading progress for book')).toBeInTheDocument()
  })
})
