import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { ItemSchema, FeedSchema } from '@/lib/gen/feeds/v1/feeds_pb'

const mockUseFeeds = jest.fn()
const mockUseFeedItems = jest.fn()
const mockUseFetchFeedItemsPage = jest.fn()

jest.mock('@/hooks/useFeeds', () => ({
  useFeeds: () => mockUseFeeds(),
  useFeedItems: (unreadOnly: boolean) => mockUseFeedItems(unreadOnly),
  useFetchFeedItemsPage: (unreadOnly: boolean) => mockUseFetchFeedItemsPage(unreadOnly),
  useUpdateItem: () => jest.fn()
}))

jest.mock(
  '@/components/feeds/ArticleReaderDialog',
  () => (props: { open: boolean; item: { id: string }; onMarkRead: (itemId: string) => void }) =>
    props.open ? (
      <div data-testid="reader-open">
        <button onClick={() => props.onMarkRead(props.item.id)}>Mark read</button>
      </div>
    ) : null
)

import FeedReaderClient from '@/components/feeds/FeedReaderClient'

function item(
  id: string,
  overrides: Partial<{ readAt: string; dismissed: boolean; createdAt: string }> = {}
) {
  return create(ItemSchema, {
    id,
    feedId: 'feed-1',
    title: `Item ${id}`,
    readAt: '',
    dismissed: false,
    publishedAt: '2026-07-01T00:00:00Z',
    contentHtml: '<p>content</p>',
    ...overrides
  })
}

describe('FeedReaderClient', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    window.localStorage.clear()
    mockUseFeeds.mockReturnValue({
      data: { feeds: [create(FeedSchema, { id: 'feed-1', title: 'Example Blog' })] }
    })
    mockUseFetchFeedItemsPage.mockReturnValue(jest.fn())
  })

  it('shows a loading state', () => {
    mockUseFeedItems.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    render(<FeedReaderClient />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockUseFeedItems.mockReturnValue({
      data: undefined,
      error: new Error('nope'),
      isLoading: false
    })
    render(<FeedReaderClient />)
    expect(screen.getByText('Failed to load feed items.')).toBeInTheDocument()
  })

  it('shows an empty state when there are no unread items', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)
    expect(screen.getByText('No unread feed items.')).toBeInTheDocument()
  })

  it('defaults to querying unread-only items', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)
    expect(mockUseFeedItems).toHaveBeenCalledWith(true)
  })

  it('lists items with their feed title (server already filters read/dismissed)', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    expect(screen.getByText('Item 1')).toBeInTheDocument()
    expect(screen.getByText('Example Blog')).toBeInTheDocument()
  })

  it('shows a no-content hint for items without stored HTML', () => {
    const noContentItem = create(ItemSchema, { id: '4', title: 'Item 4', contentHtml: '' })
    mockUseFeedItems.mockReturnValue({
      data: { items: [noContentItem], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    expect(screen.getByText('No in-app content')).toBeInTheDocument()
  })

  it('badges items ingested since the last visit as New, but not older unread ones', () => {
    window.localStorage.setItem('feeds:lastVisit', '1782000000000') // 2026-06-19T...
    mockUseFeedItems.mockReturnValue({
      data: {
        items: [
          item('1', { createdAt: '2020-01-01T00:00:00Z' }),
          item('2', { createdAt: '2030-01-01T00:00:00Z' })
        ],
        hasMore: false
      },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    expect(screen.queryByText('New')).toBeInTheDocument()
    expect(screen.getAllByText('New')).toHaveLength(1)
  })

  it('opens the reader dialog when an item title is clicked', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    expect(screen.queryByTestId('reader-open')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Item 1' }))
    expect(screen.getByTestId('reader-open')).toBeInTheDocument()
  })

  it('keeps an item visible after marking read, before the undo window elapses', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    const { rerender } = render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Item 1' }))
    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))

    // Simulate the SWR revalidation triggered by the mutation resolving: the
    // unread-only fetch no longer includes this item server-side.
    mockUseFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    rerender(<FeedReaderClient />)

    expect(screen.getByText('Item 1')).toBeInTheDocument()
  })

  it('toggles to show read items and back, switching the unread_only query', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Show read items' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(false)

    fireEvent.click(screen.getByRole('button', { name: 'Show unread only' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(true)
  })

  it('loads the next page on Load more', async () => {
    const fetchPage = jest.fn().mockResolvedValue({ items: [item('2')], hasMore: false })
    mockUseFetchFeedItemsPage.mockReturnValue(fetchPage)
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: true },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    expect(screen.getByText('Item 1')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))

    await waitFor(() => expect(screen.getByText('Item 2')).toBeInTheDocument())
    expect(fetchPage).toHaveBeenCalledWith(1)
  })
})
