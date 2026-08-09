import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { ItemSchema, FeedSchema } from '@/lib/gen/feeds/v1/feeds_pb'

const mockUseFeeds = jest.fn()
const mockUseFeedItems = jest.fn()
const mockUseFetchFeedItemsPage = jest.fn()

jest.mock('@/hooks/useFeeds', () => ({
  useFeeds: () => mockUseFeeds(),
  useFeedItems: (unreadOnly: boolean, feedId?: string, bookmarkedOnly?: boolean) =>
    mockUseFeedItems(unreadOnly, feedId, bookmarkedOnly),
  useFetchFeedItemsPage: (unreadOnly: boolean, feedId?: string, bookmarkedOnly?: boolean) =>
    mockUseFetchFeedItemsPage(unreadOnly, feedId, bookmarkedOnly),
  useUpdateItem: () => jest.fn()
}))

jest.mock(
  '@/components/feeds/ArticleReaderDialog',
  () =>
    (props: {
      open: boolean
      item: { id: string }
      onOpenChange: (open: boolean) => void
      onMarkRead: (itemId: string) => void
      onSettled: (itemId: string) => void
    }) => (
      <div data-testid={props.open ? 'reader-open' : 'reader-closed'}>
        <button onClick={() => props.onMarkRead(props.item.id)}>Mark read</button>
        <button onClick={() => props.onSettled(props.item.id)}>Settle</button>
        <button onClick={() => props.onOpenChange(false)}>Close reader</button>
      </div>
    )
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
    expect(mockUseFeedItems).toHaveBeenCalledWith(true, undefined, false)
  })

  it('filters by the selected feed', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'feed-1' } })
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(true, 'feed-1', false)

    fireEvent.change(screen.getByRole('combobox'), { target: { value: '' } })
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(true, undefined, false)
  })

  it('lists items with their feed title (server already filters read/dismissed)', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    expect(screen.getByText('Item 1')).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Example Blog' })).toBeInTheDocument()
    expect(screen.getAllByText('Example Blog')).toHaveLength(2)
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

  it('keeps the reader open when the undo window settles while it is still open (#863)', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    const { rerender } = render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Item 1' }))
    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))

    // Simulate the SWR revalidation dropping the item server-side, then the
    // undo window elapsing while the reader is still open.
    mockUseFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    rerender(<FeedReaderClient />)
    fireEvent.click(screen.getByRole('button', { name: 'Settle' }))
    rerender(<FeedReaderClient />)

    expect(screen.getByTestId('reader-open')).toBeInTheDocument()
    expect(screen.getByText('Item 1').closest('.rounded-2xl')).toHaveClass('opacity-60')

    // Closing the reader now flushes the deferred settle, hiding the item.
    fireEvent.click(screen.getByRole('button', { name: 'Close reader' }))
    expect(screen.queryByText('Item 1')).not.toBeInTheDocument()
  })

  it('hides a read item as soon as the reader closes, without waiting out the undo window (#913)', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    const { rerender } = render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Item 1' }))
    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))

    // The mutation's revalidation drops the item from the unread-only fetch.
    mockUseFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    rerender(<FeedReaderClient />)
    expect(screen.getByText('Item 1')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Close reader' }))
    expect(screen.queryByText('Item 1')).not.toBeInTheDocument()
  })

  it('settles a late undo window without error once the reader is already closed', () => {
    // The item stays in the fetched page (e.g. the show-read view), so its
    // card — and its undo timer — outlive the reader being closed.
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Item 1' }))
    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))
    fireEvent.click(screen.getByRole('button', { name: 'Close reader' }))
    fireEvent.click(screen.getByRole('button', { name: 'Settle' }))

    expect(screen.getByText('Item 1').closest('.rounded-2xl')).not.toHaveClass('opacity-60')
  })

  it('dims already-read items but not unread ones', () => {
    mockUseFeedItems.mockReturnValue({
      data: {
        items: [item('1', { readAt: '2026-07-02T00:00:00Z' }), item('2')],
        hasMore: false
      },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    expect(screen.getByText('Item 1').closest('.rounded-2xl')).toHaveClass('opacity-60')
    expect(screen.getByText('Item 2').closest('.rounded-2xl')).not.toHaveClass('opacity-60')
  })

  it('dims an item as soon as it is marked read, before the undo window elapses', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    const { rerender } = render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Item 1' }))
    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))

    mockUseFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    rerender(<FeedReaderClient />)

    expect(screen.getByText('Item 1').closest('.rounded-2xl')).toHaveClass('opacity-60')
  })

  it('toggles to show read items and back, switching the unread_only query', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Show read items' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(false, undefined, false)

    fireEvent.click(screen.getByRole('button', { name: 'Show unread only' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(true, undefined, false)
  })

  it('toggles to show bookmarked items and back, switching the bookmarked_only query', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Show bookmarked' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(false, undefined, true)

    fireEvent.click(screen.getByRole('button', { name: 'Show all' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(true, undefined, false)
  })

  it('keeps read items in the bookmarked view and hides the read/unread toggle', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Show read items' }))
    fireEvent.click(screen.getByRole('button', { name: 'Show bookmarked' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(false, undefined, true)
    expect(screen.queryByRole('button', { name: 'Show unread only' })).toBeNull()

    // The pre-bookmark unread/read choice is restored on the way out.
    fireEvent.click(screen.getByRole('button', { name: 'Show all' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(false, undefined, false)
  })

  it('shows an empty state specific to bookmarked items when none match', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Show bookmarked' }))
    expect(screen.getByText('No bookmarked feed items.')).toBeInTheDocument()
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
