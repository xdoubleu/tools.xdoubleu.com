import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { ItemSchema, FeedSchema } from '@/lib/gen/feeds/v1/feeds_pb'

const mockUseFeeds = jest.fn()
const mockUseFeedItems = jest.fn()

jest.mock('@/hooks/useFeeds', () => ({
  useFeeds: () => mockUseFeeds(),
  useFeedItems: () => mockUseFeedItems(),
  useUpdateItem: () => jest.fn()
}))

jest.mock(
  '@/components/feeds/ArticleReaderDialog',
  () => (props: { open: boolean }) => (props.open ? <div data-testid="reader-open" /> : null)
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
    mockUseFeedItems.mockReturnValue({ data: { items: [] }, error: undefined, isLoading: false })
    render(<FeedReaderClient />)
    expect(screen.getByText('No unread feed items.')).toBeInTheDocument()
  })

  it('lists unread items with their feed title, filtering out read/dismissed ones', () => {
    mockUseFeedItems.mockReturnValue({
      data: {
        items: [
          item('1'),
          item('2', { readAt: '2026-07-02T00:00:00Z' }),
          item('3', { dismissed: true })
        ]
      },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    expect(screen.getByText('Item 1')).toBeInTheDocument()
    expect(screen.queryByText('Item 2')).not.toBeInTheDocument()
    expect(screen.queryByText('Item 3')).not.toBeInTheDocument()
    expect(screen.getByText('Example Blog')).toBeInTheDocument()
  })

  it('shows a no-content hint for items without stored HTML', () => {
    const noContentItem = create(ItemSchema, { id: '4', title: 'Item 4', contentHtml: '' })
    mockUseFeedItems.mockReturnValue({
      data: { items: [noContentItem] },
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
        ]
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
      data: { items: [item('1')] },
      error: undefined,
      isLoading: false
    })
    render(<FeedReaderClient />)

    expect(screen.queryByTestId('reader-open')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Item 1' }))
    expect(screen.getByTestId('reader-open')).toBeInTheDocument()
  })
})
