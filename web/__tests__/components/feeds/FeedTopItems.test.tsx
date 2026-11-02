import { fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { FeedSchema, ItemSchema } from '@/lib/gen/feeds/v1/feeds_pb'
import FeedTopItems from '@/components/feeds/FeedTopItems'

const mockTop = vi.fn()
vi.mock('@/hooks/useFeedExpiry', () => ({ useTopFeedItems: (e: boolean) => mockTop(e) }))
vi.mock('@/hooks/useFeeds', () => ({
  useFeeds: () => ({ data: { feeds: [create(FeedSchema, { id: 'f1', title: 'Blog' })] } }),
  useUpdateItem: () => vi.fn()
}))
vi.mock('@/components/feeds/ArticleReaderDialog', () => ({
  default: (props: { item: { id: string }; onMarkRead: (id: string) => void }) => (
    <button onClick={() => props.onMarkRead(props.item.id)}>Mark {props.item.id} read</button>
  )
}))

const item = (id: string, readAt = '') =>
  create(ItemSchema, { id, feedId: 'f1', title: `Item ${id}`, readAt, hasContent: true })

describe('FeedTopItems', () => {
  it('shows loading and error states', () => {
    mockTop.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    const { rerender } = render(<FeedTopItems />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()

    mockTop.mockReturnValue({ data: undefined, error: new Error('x'), isLoading: false })
    rerender(<FeedTopItems />)
    expect(screen.getByText("Failed to load today's top items.")).toBeInTheDocument()
  })

  it('numbers the items and ends the day once all are read', () => {
    mockTop.mockReturnValue({
      data: { items: [item('a'), item('b')] },
      error: undefined,
      isLoading: false
    })
    render(<FeedTopItems />)
    expect(screen.getByText('#1')).toBeInTheDocument()
    expect(screen.getByText('#2')).toBeInTheDocument()
    expect(screen.getAllByText('Blog', { selector: 'p' })).toHaveLength(2)
    expect(screen.queryByText('You’re done for today.')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Mark a read' }))
    expect(screen.queryByText('You’re done for today.')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Mark b read' }))
    expect(screen.getByText('You’re done for today.')).toBeInTheDocument()
  })

  it('says so when there is nothing unread', () => {
    mockTop.mockReturnValue({ data: { items: [] }, error: undefined, isLoading: false })
    render(<FeedTopItems />)
    expect(screen.getByText('Nothing unread today.')).toBeInTheDocument()
  })

  it('treats items already read today as done', () => {
    mockTop.mockReturnValue({
      data: { items: [item('a', '2026-11-02T08:00:00Z')] },
      error: undefined,
      isLoading: false
    })
    render(<FeedTopItems />)
    expect(screen.getByText('You’re done for today.')).toBeInTheDocument()
  })
})
