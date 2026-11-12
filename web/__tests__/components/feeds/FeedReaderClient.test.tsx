import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { ItemSchema, FeedSchema } from '@/lib/gen/feeds/v1/feeds_pb'

const mockUseFeeds = vi.fn()
const mockUseFeedItems = vi.fn()
const mockUseFetchFeedItemsPage = vi.fn()

vi.mock('@/hooks/useFeeds', () => ({
  usePrefetchFeedBodies: () => {},
  useFeeds: () => mockUseFeeds(),
  useFeedItems: (unreadOnly: boolean, feedId?: string, bookmarkedOnly?: boolean) =>
    mockUseFeedItems(unreadOnly, feedId, bookmarkedOnly),
  useFetchFeedItemsPage: (unreadOnly: boolean, feedId?: string, bookmarkedOnly?: boolean) =>
    mockUseFetchFeedItemsPage(unreadOnly, feedId, bookmarkedOnly),
  useUpdateItem: () => vi.fn()
}))

const mockLastChance = vi.fn()
const mockTopItems = vi.fn()
vi.mock('@/hooks/useFeedExpiry', () => ({
  useLastChanceFeedItems: (enabled: boolean) => mockLastChance(enabled),
  useTopFeedItems: (enabled: boolean) => mockTopItems(enabled)
}))

vi.mock('@/components/feeds/ArticleReaderDialog', () => ({
  default: (props: {
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
}))

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
    vi.clearAllMocks()
    window.localStorage.clear()
    mockUseFeeds.mockReturnValue({
      data: { feeds: [create(FeedSchema, { id: 'feed-1', title: 'Example Blog' })] }
    })
    mockUseFetchFeedItemsPage.mockReturnValue(vi.fn())
    mockLastChance.mockReturnValue({ data: undefined })
    mockTopItems.mockReturnValue({ data: undefined, error: undefined, isLoading: false })
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

  it('points to Manage feeds when there are no feeds yet', () => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [] } })
    mockUseFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)
    expect(screen.getByText(/No feeds yet\. Use Manage feeds/)).toBeInTheDocument()
  })

  it("shows an item's categories as chips", () => {
    mockUseFeedItems.mockReturnValue({
      data: {
        items: [create(ItemSchema, { id: 'a', title: 'Item a', categories: ['Claude Code'] })],
        hasMore: false
      },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)
    expect(screen.getByRole('list', { name: 'Categories' })).toHaveTextContent('Claude Code')
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

    expect(screen.getByRole('combobox')).toHaveValue('')
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual([
      'All feeds',
      'Example Blog'
    ])
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'feed-1' } })
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(true, 'feed-1', false)
    expect(screen.getByRole('combobox')).toHaveValue('feed-1')

    fireEvent.change(screen.getByRole('combobox'), { target: { value: '' } })
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(true, undefined, false)
  })

  it('lists only the empty choice while feeds are loading', () => {
    mockUseFeeds.mockReturnValue({ data: undefined })
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)
    expect(screen.getAllByRole('option')).toHaveLength(1)
    expect(screen.getByText('Item 1')).toBeInTheDocument()
  })

  it('picks up feed titles that load after the items', () => {
    mockUseFeeds.mockReturnValue({ data: { feeds: [] } })
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    const { rerender } = render(<FeedReaderClient />)
    mockUseFeeds.mockReturnValue({
      data: { feeds: [create(FeedSchema, { id: 'feed-1', title: 'Late Blog' })] }
    })
    rerender(<FeedReaderClient />)
    expect(screen.getAllByText('Late Blog')).toHaveLength(2)
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

    // Revalidation drops the item from the unread-only fetch.
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

    // Revalidation drops the item, then the undo window elapses mid-read.
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
    // The item stays in the page, so its card and undo timer outlive the reader.
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

  it('switches between unread and all items, changing the unread_only query', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)
    expect(screen.getByRole('tab', { name: 'Unread' })).toHaveAttribute('aria-selected', 'true')

    fireEvent.click(screen.getByRole('tab', { name: 'All' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(false, undefined, false)

    fireEvent.click(screen.getByRole('tab', { name: 'Unread' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(true, undefined, false)
  })

  it('shows bookmarked items, read or not, and back', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [item('1')], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('tab', { name: 'Bookmarked' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(false, undefined, true)

    fireEvent.click(screen.getByRole('tab', { name: 'Unread' }))
    expect(mockUseFeedItems).toHaveBeenLastCalledWith(true, undefined, false)
  })

  it('shows an empty state for all items when none match', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)
    fireEvent.click(screen.getByRole('tab', { name: 'All' }))
    expect(screen.getByText('No feed items.')).toBeInTheDocument()
  })

  it('shows an empty state specific to bookmarked items when none match', () => {
    mockUseFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    fireEvent.click(screen.getByRole('tab', { name: 'Bookmarked' }))
    expect(screen.getByText('No bookmarked feed items.')).toBeInTheDocument()
  })

  it('loads the next page on Load more', async () => {
    const fetchPage = vi.fn().mockResolvedValue({ items: [item('2')], hasMore: false })
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

  describe('last chance strip', () => {
    const inbox = () =>
      mockUseFeedItems.mockReturnValue({
        data: { items: [item('a'), item('b')], hasMore: false },
        error: undefined,
        isLoading: false
      })

    it('shows items about to expire above the list and out of the grid', () => {
      inbox()
      const soon = new Date(Date.now() + 3 * 3_600_000).toISOString()
      mockLastChance.mockReturnValue({
        data: { items: [create(ItemSchema, { ...item('a'), expiresAt: soon })] }
      })
      render(<FeedReaderClient />)

      const strip = screen.getByRole('region', { name: 'Last chance' })
      expect(within(strip).getByText('Item a')).toBeInTheDocument()
      expect(within(strip).getByText('Expires in 2h')).toBeInTheDocument()
      // Item a appears once (strip), item b stays in the grid.
      expect(screen.getAllByText('Item a')).toHaveLength(1)
      expect(screen.getByText('Item b')).toBeInTheDocument()
    })

    it('only loads for the plain unread inbox', () => {
      inbox()
      render(<FeedReaderClient />)
      expect(mockLastChance).toHaveBeenLastCalledWith(true)

      fireEvent.click(screen.getByRole('tab', { name: 'All' }))
      expect(mockLastChance).toHaveBeenLastCalledWith(false)
      expect(screen.queryByRole('region', { name: 'Last chance' })).not.toBeInTheDocument()
    })

    it('narrows to the selected feed', () => {
      inbox()
      mockLastChance.mockReturnValue({
        data: { items: [create(ItemSchema, { ...item('a'), feedId: 'feed-2' })] }
      })
      render(<FeedReaderClient />)
      expect(screen.getByRole('region', { name: 'Last chance' })).toBeInTheDocument()

      fireEvent.change(screen.getByLabelText('Filter by feed'), { target: { value: 'feed-1' } })
      expect(screen.queryByRole('region', { name: 'Last chance' })).not.toBeInTheDocument()
    })
  })

  describe("today's top view", () => {
    it('swaps the inbox for the ranked list and back', () => {
      mockUseFeedItems.mockReturnValue({
        data: { items: [item('a')], hasMore: false },
        error: undefined,
        isLoading: false
      })
      mockTopItems.mockReturnValue({
        data: { items: [item('t1'), item('t2', { readAt: '2026-07-02T00:00:00Z' })] },
        error: undefined,
        isLoading: false
      })
      render(<FeedReaderClient />)

      fireEvent.click(screen.getByRole('tab', { name: 'Today’s top 10' }))
      expect(screen.queryByLabelText('Filter by feed')).not.toBeInTheDocument()
      expect(screen.getByText('Item t1')).toBeInTheDocument()
      expect(screen.getByText('#2')).toBeInTheDocument()
      expect(screen.queryByText('Item a')).not.toBeInTheDocument()
      expect(screen.queryByText('You’re done for today.')).not.toBeInTheDocument()

      fireEvent.click(screen.getByRole('tab', { name: 'Unread' }))
      expect(screen.getByText('Item a')).toBeInTheDocument()
    })
  })
})
