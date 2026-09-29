import { act, fireEvent, render, screen, within } from '@testing-library/react'
import FeedFilteredItemsClient from '@/components/feeds/FeedFilteredItemsClient'
import { FilterRuleKind } from '@/lib/gen/feeds/v1/feeds_pb'

const mockUseFeeds = jest.fn()
jest.mock('@/hooks/useFeeds', () => ({
  useFeeds: () => mockUseFeeds()
}))

const mockUseFilteredFeedItems = jest.fn()
const mockFetchPage = jest.fn()
const mockRestore = jest.fn()
jest.mock('@/hooks/useFeedFilteredItems', () => ({
  useFilteredFeedItems: (feedId?: string) => mockUseFilteredFeedItems(feedId),
  useFetchFilteredFeedItemsPage: () => mockFetchPage,
  useRestoreFeedItem: () => mockRestore
}))

const feeds = [
  { id: 'feed-1', title: 'Claude blog', url: 'https://claude.com/blog' },
  { id: 'feed-2', title: '', url: 'https://example.com/feed.xml' }
]

const items = [
  {
    id: 'item-1',
    feedId: 'feed-1',
    title: 'New model launch',
    sourceUrl: 'https://claude.com/blog/launch',
    categories: ['Product announcements'],
    filteredAt: '2026-09-01T10:00:00Z',
    filterRule: {
      id: 'rule-1',
      feedId: 'feed-1',
      kind: FilterRuleKind.CATEGORY,
      value: 'Product announcements'
    }
  },
  {
    id: 'item-2',
    feedId: 'feed-2',
    title: 'A sponsored post',
    sourceUrl: 'https://example.com/sponsored',
    categories: [],
    filteredAt: '2026-08-30T10:00:00Z',
    filterRule: { id: 'rule-2', feedId: '', kind: FilterRuleKind.TITLE, value: 'sponsored' }
  },
  {
    id: 'item-3',
    feedId: 'feed-2',
    title: 'Orphaned item',
    sourceUrl: 'https://example.com/orphan',
    categories: [],
    filteredAt: '2026-08-29T10:00:00Z',
    filterRule: undefined
  }
]

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

const row = (title: string) => screen.getByText(title).closest('li')!

describe('FeedFilteredItemsClient', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUseFeeds.mockReturnValue({ data: { feeds } })
    mockUseFilteredFeedItems.mockReturnValue({
      data: { items, hasMore: false },
      error: undefined,
      isLoading: false
    })
  })

  it('shows each item with its link, feed, categories, reason and filtered date', () => {
    render(<FeedFilteredItemsClient />)

    const first = row('New model launch')
    const link = within(first).getByRole('link', { name: 'New model launch' })
    expect(link).toHaveAttribute('href', 'https://claude.com/blog/launch')
    expect(link).toHaveAttribute('target', '_blank')
    expect(first).toHaveTextContent('Claude blog')
    expect(within(first).getByText('Product announcements', { selector: 'span' })).toBeVisible()
    expect(first).toHaveTextContent('Category is Product announcements · Claude blog')
    expect(first).toHaveTextContent('Filtered 01/09/2026')

    const second = row('A sponsored post')
    expect(second).toHaveTextContent('https://example.com/feed.xml')
    expect(second).toHaveTextContent('Title contains sponsored · All feeds')

    expect(row('Orphaned item')).toHaveTextContent('Rule deleted')

    const button = within(first).getByRole('button', { name: 'Restore New model launch' })
    expect(button).toHaveTextContent(/^Restore$/)
    expect(screen.queryByText('Restoring failed. Please try again.')).not.toBeInTheDocument()
    expect(screen.queryByText('No filtered items.')).not.toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Load more/i })).not.toBeInTheDocument()
  })

  it('labels a feed it no longer knows, and relabels once feeds load', () => {
    mockUseFeeds.mockReturnValue({ data: undefined })
    const { rerender } = render(<FeedFilteredItemsClient />)
    expect(row('New model launch')).toHaveTextContent(
      'Category is Product announcements · Unknown feed'
    )
    expect(within(screen.getByLabelText('Filter by feed')).getAllByRole('option')).toHaveLength(1)

    mockUseFeeds.mockReturnValue({ data: { feeds } })
    rerender(<FeedFilteredItemsClient />)
    expect(row('New model launch')).toHaveTextContent(
      'Category is Product announcements · Claude blog'
    )
  })

  it('shows refreshed items when the list revalidates', () => {
    const { rerender } = render(<FeedFilteredItemsClient />)
    mockUseFilteredFeedItems.mockReturnValue({
      data: { items: items.slice(2), hasMore: false },
      error: undefined,
      isLoading: false
    })
    rerender(<FeedFilteredItemsClient />)
    expect(screen.queryByText('New model launch')).not.toBeInTheDocument()
    expect(screen.getByText('Orphaned item')).toBeInTheDocument()
  })

  it('lists the chosen feed only', () => {
    render(<FeedFilteredItemsClient />)
    expect(mockUseFilteredFeedItems).toHaveBeenLastCalledWith(undefined)

    fireEvent.change(screen.getByLabelText('Filter by feed'), { target: { value: 'feed-2' } })
    expect(mockUseFilteredFeedItems).toHaveBeenLastCalledWith('feed-2')

    fireEvent.change(screen.getByLabelText('Filter by feed'), { target: { value: '' } })
    expect(mockUseFilteredFeedItems).toHaveBeenLastCalledWith(undefined)
  })

  it('starts on the feed it was opened for', () => {
    render(<FeedFilteredItemsClient initialFeedId="feed-1" />)
    expect(mockUseFilteredFeedItems).toHaveBeenLastCalledWith('feed-1')
    expect(screen.getByLabelText('Filter by feed')).toHaveValue('feed-1')
  })

  it('restores an item, removing its row and confirming', async () => {
    const pending = deferred<unknown>()
    mockRestore.mockReturnValueOnce(pending.promise)
    render(<FeedFilteredItemsClient />)

    fireEvent.click(within(row('New model launch')).getByRole('button', { name: /Restore/ }))
    expect(mockRestore).toHaveBeenCalledWith('item-1')
    expect(within(row('New model launch')).getByRole('button')).toHaveTextContent('Restoring…')
    expect(within(row('New model launch')).getByRole('button')).toBeDisabled()

    await act(async () => pending.resolve({ item: { id: 'item-1' } }))

    expect(screen.queryByText('New model launch')).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent(
      'Restored “New model launch” to the inbox.'
    )
  })

  it('keeps the row and says so when restoring fails', async () => {
    mockRestore.mockRejectedValueOnce(new Error('boom'))
    render(<FeedFilteredItemsClient />)

    await act(async () => {
      fireEvent.click(within(row('Orphaned item')).getByRole('button', { name: /Restore/ }))
    })

    const orphan = row('Orphaned item')
    expect(orphan).toHaveTextContent('Restoring failed. Please try again.')
    expect(within(orphan).getByRole('button', { name: /Restore/ })).toBeEnabled()

    const retry = deferred<unknown>()
    mockRestore.mockReturnValueOnce(retry.promise)
    fireEvent.click(within(orphan).getByRole('button', { name: /Restore/ }))
    expect(orphan).not.toHaveTextContent('Restoring failed.')
    await act(async () => retry.resolve({}))
  })

  it('shows an empty state', () => {
    mockUseFilteredFeedItems.mockReturnValue({
      data: { items: [], hasMore: false },
      error: undefined,
      isLoading: false
    })
    render(<FeedFilteredItemsClient />)
    expect(screen.getByText('No filtered items.')).toBeInTheDocument()
  })

  it('shows loading and error states', () => {
    mockUseFilteredFeedItems.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    const { rerender } = render(<FeedFilteredItemsClient />)
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Loading filtered items…')
    expect(screen.queryByText('No filtered items.')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Load more/i })).not.toBeInTheDocument()

    mockUseFilteredFeedItems.mockReturnValue({
      data: undefined,
      error: new Error('x'),
      isLoading: false
    })
    rerender(<FeedFilteredItemsClient />)
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load filtered items.')
    expect(screen.queryByText('No filtered items.')).not.toBeInTheDocument()
  })

  it('loads more pages', async () => {
    mockUseFilteredFeedItems.mockReturnValue({
      data: { items: items.slice(0, 1), hasMore: true },
      error: undefined,
      isLoading: false
    })
    mockFetchPage.mockResolvedValueOnce({ items: items.slice(1, 2), hasMore: false })
    render(<FeedFilteredItemsClient />)

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Load more/i }))
    })

    expect(mockFetchPage).toHaveBeenCalledWith(1)
    expect(screen.getByText('A sponsored post')).toBeInTheDocument()
  })

  it('keeps loaded pages when the first page revalidates unchanged, and resets otherwise', async () => {
    mockUseFilteredFeedItems.mockReturnValue({
      data: { items: items.slice(0, 1), hasMore: true },
      error: undefined,
      isLoading: false
    })
    mockFetchPage.mockResolvedValueOnce({ items: items.slice(1, 2), hasMore: false })
    const { rerender } = render(<FeedFilteredItemsClient />)
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Load more/i }))
    })

    mockUseFilteredFeedItems.mockReturnValue({
      data: { items: [{ ...items[0] }], hasMore: true },
      error: undefined,
      isLoading: false
    })
    rerender(<FeedFilteredItemsClient />)
    expect(screen.getByText('A sponsored post')).toBeInTheDocument()

    mockUseFilteredFeedItems.mockReturnValue({
      data: { items: items.slice(2), hasMore: true },
      error: undefined,
      isLoading: false
    })
    rerender(<FeedFilteredItemsClient />)
    expect(screen.queryByText('A sponsored post')).not.toBeInTheDocument()
    expect(screen.getByText('Orphaned item')).toBeInTheDocument()
  })
})
