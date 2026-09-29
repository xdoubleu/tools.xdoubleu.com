import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { ConnectError, Code } from '@connectrpc/connect'
import FeedFilterRulesCard from '@/components/feeds/FeedFilterRulesCard'
import { FilterRuleKind } from '@/lib/gen/feeds/v1/feeds_pb'

const mockUseFeeds = jest.fn()
const mockUseFilterRules = jest.fn()
const mockCreate = jest.fn()
const mockDelete = jest.fn()
jest.mock('@/hooks/useFeeds', () => ({
  useFeeds: () => mockUseFeeds(),
  useFilterRules: () => mockUseFilterRules(),
  useCreateFilterRule: () => mockCreate,
  useDeleteFilterRule: () => mockDelete
}))

const feeds = [
  { id: 'feed-1', title: 'Claude blog', url: 'https://claude.com/blog' },
  { id: 'feed-2', title: '', url: 'https://example.com/feed.xml' }
]

const rules = [
  {
    id: 'rule-1',
    feedId: 'feed-1',
    kind: FilterRuleKind.CATEGORY,
    value: 'Product announcements',
    filteredCount: 7
  },
  { id: 'rule-2', feedId: '', kind: FilterRuleKind.TITLE, value: 'sponsored', filteredCount: 0 }
]

describe('FeedFilterRulesCard', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUseFeeds.mockReturnValue({ data: { feeds } })
    mockUseFilterRules.mockReturnValue({ data: { rules }, error: undefined, isLoading: false })
  })

  it('lists each rule with its scope, kind, value and filtered count', () => {
    render(<FeedFilterRulesCard />)

    const [first, second] = screen.getAllByRole('listitem')
    expect(first).toHaveTextContent('Claude blog')
    expect(first).toHaveTextContent('Category')
    expect(first).toHaveTextContent('Product announcements')
    expect(first).toHaveTextContent('7 filtered')
    expect(second).toHaveTextContent('All feeds')
    expect(second).toHaveTextContent('Title contains')
    expect(second).toHaveTextContent('sponsored')
    expect(second).toHaveTextContent('0 filtered')
  })

  it('offers All feeds plus every feed as the scope', () => {
    render(<FeedFilterRulesCard />)
    const options = within(screen.getByLabelText('Applies to')).getAllByRole('option')
    expect(options.map((o) => o.textContent)).toEqual([
      'All feeds',
      'Claude blog',
      'https://example.com/feed.xml'
    ])
  })

  it('creates a rule and reports how many existing items it filtered', async () => {
    mockCreate.mockResolvedValue({ rule: { id: 'rule-3', filteredCount: 7 } })
    render(<FeedFilterRulesCard />)

    fireEvent.change(screen.getByLabelText('Applies to'), { target: { value: 'feed-1' } })
    fireEvent.change(screen.getByLabelText('Match'), {
      target: { value: String(FilterRuleKind.CATEGORY) }
    })
    fireEvent.change(screen.getByLabelText('Value'), { target: { value: 'Product announcements' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(mockCreate).toHaveBeenCalledWith({
      feedId: 'feed-1',
      kind: FilterRuleKind.CATEGORY,
      value: 'Product announcements'
    })
    expect(await screen.findByText('Filtered 7 existing items.')).toBeInTheDocument()
    expect(screen.getByLabelText('Value')).toHaveValue('')
  })

  it('defaults to a title rule on all feeds and says 1 item in the singular', async () => {
    mockCreate.mockResolvedValue({ rule: { id: 'rule-3', filteredCount: 1 } })
    render(<FeedFilterRulesCard />)

    fireEvent.change(screen.getByLabelText('Value'), { target: { value: 'webinar' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(mockCreate).toHaveBeenCalledWith({
      feedId: '',
      kind: FilterRuleKind.TITLE,
      value: 'webinar'
    })
    expect(await screen.findByText('Filtered 1 existing item.')).toBeInTheDocument()
  })

  it('says so when a new rule matched no existing items', async () => {
    mockCreate.mockResolvedValue({ rule: { id: 'rule-3', filteredCount: 0 } })
    render(<FeedFilterRulesCard />)

    fireEvent.change(screen.getByLabelText('Value'), { target: { value: 'rare' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(await screen.findByText('Rule added. No existing items matched.')).toBeInTheDocument()
  })

  it('disables Add rule until a value is typed', () => {
    render(<FeedFilterRulesCard />)
    expect(screen.getByRole('button', { name: 'Add rule' })).toBeDisabled()
  })

  it('explains a duplicate rule', async () => {
    mockCreate.mockRejectedValue(new ConnectError('dup', Code.AlreadyExists))
    render(<FeedFilterRulesCard />)

    fireEvent.change(screen.getByLabelText('Value'), { target: { value: 'sponsored' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(await screen.findByText('That rule already exists.')).toBeInTheDocument()
  })

  it('reports other create failures generically', async () => {
    mockCreate.mockRejectedValue(new Error('boom'))
    render(<FeedFilterRulesCard />)

    fireEvent.change(screen.getByLabelText('Value'), { target: { value: 'x' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(await screen.findByText('Adding the rule failed. Please try again.')).toBeInTheDocument()
  })

  it('deletes a rule', async () => {
    mockDelete.mockResolvedValue(undefined)
    render(<FeedFilterRulesCard />)

    fireEvent.click(
      screen.getByRole('button', { name: 'Delete rule Category: Product announcements' })
    )
    await waitFor(() => expect(mockDelete).toHaveBeenCalledWith('rule-1'))
  })

  it('reports a failed delete', async () => {
    mockDelete.mockRejectedValue(new Error('boom'))
    render(<FeedFilterRulesCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Delete rule Title contains: sponsored' }))
    expect(await screen.findByText('Deleting the rule failed.')).toBeInTheDocument()
  })

  it('shows an empty state', () => {
    mockUseFilterRules.mockReturnValue({ data: { rules: [] }, error: undefined, isLoading: false })
    render(<FeedFilterRulesCard />)
    expect(screen.getByText('No filter rules yet.')).toBeInTheDocument()
  })

  it('shows loading and error states', () => {
    mockUseFilterRules.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    const { rerender } = render(<FeedFilterRulesCard />)
    expect(screen.getByText('Loading filter rules…')).toBeInTheDocument()

    mockUseFilterRules.mockReturnValue({ data: undefined, error: new Error(), isLoading: false })
    rerender(<FeedFilterRulesCard />)
    expect(screen.getByText('Failed to load filter rules.')).toBeInTheDocument()
  })

  it("names a feed rule's scope Unknown feed until feeds load", () => {
    mockUseFeeds.mockReturnValue({ data: undefined })
    render(<FeedFilterRulesCard />)
    expect(screen.getAllByRole('listitem')[0]).toHaveTextContent('Unknown feed')
  })
})
