import { act, fireEvent, render, screen, within } from '@testing-library/react'
import FeedExpiredItemsClient from '@/components/feeds/FeedExpiredItemsClient'

const mockUseFeeds = vi.fn()
vi.mock('@/hooks/useFeeds', () => ({ useFeeds: () => mockUseFeeds() }))

const mockUseExpired = vi.fn()
const mockFetchPage = vi.fn()
vi.mock('@/hooks/useFeedExpiry', () => ({
  useExpiredFeedItems: (feedId?: string) => mockUseExpired(feedId),
  useFetchExpiredFeedItemsPage: () => mockFetchPage
}))

const mockRestore = vi.fn()
vi.mock('@/hooks/useFeedFilteredItems', () => ({ useRestoreFeedItem: () => mockRestore }))

const feeds = [
  { id: 'feed-1', title: 'Claude blog', url: 'https://claude.com/blog' },
  { id: 'feed-2', title: '', url: 'https://example.com/feed.xml' }
]
const items = [
  {
    id: 'item-1',
    feedId: 'feed-1',
    title: 'Old launch post',
    sourceUrl: 'https://claude.com/blog/launch',
    categories: ['Product'],
    createdAt: '2026-10-20T10:00:00Z',
    expiredAt: '2026-10-27T10:00:00Z'
  },
  {
    id: 'item-2',
    feedId: 'feed-2',
    title: 'Older post',
    sourceUrl: 'https://example.com/older',
    categories: [],
    createdAt: '2026-10-19T10:00:00Z',
    expiredAt: '2026-10-26T10:00:00Z'
  }
]
const ok = { data: { items, hasMore: false }, error: undefined, isLoading: false }
const row = (title: string) => screen.getByText(title).closest('li')!

describe('FeedExpiredItemsClient', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseFeeds.mockReturnValue({ data: { feeds } })
    mockUseExpired.mockReturnValue(ok)
  })

  it('shows each item with its link, feed, categories and dates', () => {
    render(<FeedExpiredItemsClient />)
    const first = row('Old launch post')
    expect(within(first).getByRole('link', { name: 'Old launch post' })).toHaveAttribute(
      'href',
      'https://claude.com/blog/launch'
    )
    expect(first).toHaveTextContent('Claude blog')
    expect(first).toHaveTextContent(/Arrived 20 Oct( 2026)? · expired 27 Oct( 2026)?/)
    expect(row('Older post')).toHaveTextContent('https://example.com/feed.xml')
  })

  it('labels a feed it no longer knows', () => {
    mockUseFeeds.mockReturnValue({ data: undefined })
    render(<FeedExpiredItemsClient />)
    expect(row('Old launch post')).toHaveTextContent('Unknown feed')
  })

  it('lists the chosen feed only, starting on the one it was opened for', () => {
    render(<FeedExpiredItemsClient initialFeedId="feed-1" />)
    expect(mockUseExpired).toHaveBeenLastCalledWith('feed-1')
    fireEvent.change(screen.getByLabelText('Filter by feed'), { target: { value: '' } })
    expect(mockUseExpired).toHaveBeenLastCalledWith(undefined)
  })

  it('restores an item, removing its row and confirming', async () => {
    mockRestore.mockResolvedValueOnce({})
    render(<FeedExpiredItemsClient />)

    await act(async () => {
      fireEvent.click(
        within(row('Old launch post')).getByRole('button', { name: 'Restore Old launch post' })
      )
    })

    expect(mockRestore).toHaveBeenCalledWith('item-1')
    expect(screen.queryByText('Old launch post')).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Restored “Old launch post” to the inbox.')
  })

  it('keeps the row and says so when restoring fails', async () => {
    mockRestore.mockRejectedValueOnce(new Error('boom'))
    render(<FeedExpiredItemsClient />)

    await act(async () => {
      fireEvent.click(within(row('Older post')).getByRole('button', { name: /Restore/ }))
    })

    expect(row('Older post')).toHaveTextContent('Restoring failed. Please try again.')
    expect(within(row('Older post')).getByRole('button', { name: /Restore/ })).toBeEnabled()
  })

  it('shows empty, loading and error states', () => {
    mockUseExpired.mockReturnValue({ data: { items: [], hasMore: false }, isLoading: false })
    const { rerender } = render(<FeedExpiredItemsClient />)
    expect(screen.getByText('No expired items.')).toBeInTheDocument()

    mockUseExpired.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    rerender(<FeedExpiredItemsClient />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading expired items…')

    mockUseExpired.mockReturnValue({ data: undefined, error: new Error('x'), isLoading: false })
    rerender(<FeedExpiredItemsClient />)
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load expired items.')
  })

  it('loads more pages', async () => {
    mockUseExpired.mockReturnValue({
      data: { items: items.slice(0, 1), hasMore: true },
      error: undefined,
      isLoading: false
    })
    mockFetchPage.mockResolvedValueOnce({ items: items.slice(1), hasMore: false })
    render(<FeedExpiredItemsClient />)

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Load more/i }))
    })

    expect(mockFetchPage).toHaveBeenCalledWith(1)
    expect(screen.getByText('Older post')).toBeInTheDocument()
  })
})
