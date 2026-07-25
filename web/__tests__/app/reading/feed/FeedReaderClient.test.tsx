import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  LibraryResponseSchema,
  GetLibraryResponseSchema
} from '@/lib/gen/reading/v1/library_pb'
import { FeedItemBookSchema, ListFeedItemsResponseSchema } from '@/lib/gen/reading/v1/feeds_pb'

jest.mock('@/hooks/useBookFeeds', () => ({
  useFeedItemBooks: jest.fn()
}))

const mockUpdateBookStatus = jest.fn()
jest.mock('@/hooks/useBooks', () => ({
  useLibrary: jest.fn(),
  useUpdateBookStatus: () => mockUpdateBookStatus
}))

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: jest.fn()
}))

import FeedReaderClient from '@/app/reading/feed/FeedReaderClient'
import { useLibrary } from '@/hooks/useBooks'
import { useFeedItemBooks } from '@/hooks/useBookFeeds'

const mockUseLibrary = jest.mocked(useLibrary)
const mockUseFeedItemBooks = jest.mocked(useFeedItemBooks)

function rssItem(id: string, title: string, status = 'to-read', addedAt = '2026-01-01T00:00:00Z') {
  return create(UserBookSchema, {
    id,
    bookId: `book-${id}`,
    status,
    addedAt,
    tags: [],
    formats: [],
    book: create(BookSchema, { title, authors: [], sourceUrl: `https://example.com/${id}` })
  })
}

function mockData(
  rss: ReturnType<typeof rssItem>[],
  feedItems: { bookId: string; feedTitle: string }[] = []
) {
  // @ts-expect-error -- partial SWRResponse for test purposes
  mockUseLibrary.mockReturnValue({
    data: create(GetLibraryResponseSchema, {
      library: create(LibraryResponseSchema, { rss })
    }),
    error: undefined,
    isLoading: false
  })
  // @ts-expect-error -- partial SWRResponse for test purposes
  mockUseFeedItemBooks.mockReturnValue({
    data: create(ListFeedItemsResponseSchema, {
      items: feedItems.map((f) =>
        create(FeedItemBookSchema, { bookId: f.bookId, feedId: 'feed-1', feedTitle: f.feedTitle })
      )
    })
  })
}

describe('FeedReaderClient', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUpdateBookStatus.mockResolvedValue({})
  })

  it('shows a loading state', () => {
    // @ts-expect-error -- partial SWRResponse for test purposes
    mockUseLibrary.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    // @ts-expect-error -- partial SWRResponse for test purposes
    mockUseFeedItemBooks.mockReturnValue({ data: undefined })
    render(<FeedReaderClient />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    // @ts-expect-error -- partial SWRResponse for test purposes
    mockUseLibrary.mockReturnValue({ data: undefined, error: new Error('boom'), isLoading: false })
    // @ts-expect-error -- partial SWRResponse for test purposes
    mockUseFeedItemBooks.mockReturnValue({ data: undefined })
    render(<FeedReaderClient />)
    expect(screen.getByText('Failed to load feed items.')).toBeInTheDocument()
  })

  it('shows an empty state when there are no unread items', () => {
    mockData([])
    render(<FeedReaderClient />)
    expect(screen.getByText('No unread feed items.')).toBeInTheDocument()
  })

  it('lists unread rss items, most recent first', () => {
    mockData([
      rssItem('1', 'Older Post', 'to-read', '2026-01-01T00:00:00Z'),
      rssItem('2', 'Newer Post', 'to-read', '2026-01-02T00:00:00Z')
    ])
    render(<FeedReaderClient />)
    const titles = screen.getAllByRole('link').map((el) => el.textContent)
    expect(titles).toEqual(['Newer Post', 'Older Post'])
  })

  it('excludes already-read items', () => {
    mockData([rssItem('1', 'Read Post', 'read'), rssItem('2', 'Unread Post', 'to-read')])
    render(<FeedReaderClient />)
    expect(screen.queryByText('Read Post')).not.toBeInTheDocument()
    expect(screen.getByText('Unread Post')).toBeInTheDocument()
  })

  it('labels an item with its feed title', () => {
    mockData([rssItem('1', 'Labeled Post')], [{ bookId: 'book-1', feedTitle: 'Cool Blog' }])
    render(<FeedReaderClient />)
    expect(screen.getByText('Cool Blog')).toBeInTheDocument()
  })

  it('reverts to the row instead of removing it when Undo is clicked', async () => {
    mockData([rssItem('1', 'To Be Read')])
    render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))
    await waitFor(() => expect(mockUpdateBookStatus).toHaveBeenCalled())
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Mark read' })).toBeInTheDocument()
    })
    expect(screen.getByText('To Be Read')).toBeInTheDocument()
  })

  it('removes an item from the list once mark-read settles', async () => {
    jest.useFakeTimers({ advanceTimers: true })
    mockData([rssItem('1', 'To Be Read')])
    render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))
    await waitFor(() => expect(mockUpdateBookStatus).toHaveBeenCalled())

    act(() => {
      jest.advanceTimersByTime(4000)
    })

    await waitFor(() => {
      expect(screen.queryByText('To Be Read')).not.toBeInTheDocument()
    })
    expect(screen.getByText('No unread feed items.')).toBeInTheDocument()
    jest.useRealTimers()
  })

  it('renders a non-linked title when the item has no source URL', () => {
    const noSource = rssItem('1', 'No Source Post')
    noSource.book!.sourceUrl = ''
    mockData([noSource])
    render(<FeedReaderClient />)
    expect(screen.getByText('No Source Post').tagName).toBe('SPAN')
  })
})
