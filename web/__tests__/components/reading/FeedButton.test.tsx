import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema
} from '@/lib/gen/reading/v1/library_pb'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/hooks/useBooks', () => ({
  useLibrary: jest.fn()
}))

import FeedButton from '@/components/reading/FeedButton'
import { useLibrary } from '@/hooks/useBooks'

const mockUseLibrary = jest.mocked(useLibrary)

function rssItem(id: string, status = 'to-read') {
  return create(UserBookSchema, {
    id,
    bookId: `book-${id}`,
    status,
    tags: [],
    formats: [],
    book: create(BookSchema, { id: `book-${id}`, title: `Book ${id}`, authors: [] })
  })
}

describe('FeedButton', () => {
  it('renders without a badge while library data is loading', () => {
    // @ts-expect-error -- partial SWRResponse for test purposes
    mockUseLibrary.mockReturnValue({ data: undefined })
    render(<FeedButton />)
    expect(screen.getByRole('link', { name: /feed/i })).toHaveAttribute('href', '/reading/feed')
    expect(screen.queryByText(/^\d+$/)).not.toBeInTheDocument()
  })

  it('renders a link to /reading/feed without a badge when there are no unread items', () => {
    // @ts-expect-error -- partial SWRResponse for test purposes
    mockUseLibrary.mockReturnValue({
      data: create(GetLibraryResponseSchema, {
        library: create(LibraryResponseSchema, { rss: [] })
      })
    })
    render(<FeedButton />)
    const link = screen.getByRole('link', { name: /feed/i })
    expect(link).toHaveAttribute('href', '/reading/feed')
    expect(screen.queryByText(/^\d+$/)).not.toBeInTheDocument()
  })

  it('shows the unread count as a badge', () => {
    // @ts-expect-error -- partial SWRResponse for test purposes
    mockUseLibrary.mockReturnValue({
      data: create(GetLibraryResponseSchema, {
        library: create(LibraryResponseSchema, {
          rss: [rssItem('1', 'to-read'), rssItem('2', 'to-read'), rssItem('3', 'read')]
        })
      })
    })
    render(<FeedButton />)
    expect(screen.getByText('2')).toBeInTheDocument()
  })
})
