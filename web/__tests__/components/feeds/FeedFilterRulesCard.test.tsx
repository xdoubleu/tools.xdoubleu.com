import {
  act,
  createEvent,
  fireEvent,
  render,
  screen,
  waitFor,
  within
} from '@testing-library/react'
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
  { id: 'feed-2', title: '', url: 'https://example.com/feed.xml' },
  { id: 'feed-3', title: '', url: '' }
]

const rules = [
  {
    id: 'rule-1',
    feedId: 'feed-1',
    kind: FilterRuleKind.CATEGORY,
    value: 'Product announcements',
    filteredCount: 7
  },
  { id: 'rule-2', feedId: '', kind: FilterRuleKind.TITLE, value: 'sponsored', filteredCount: 0 },
  { id: 'rule-3', feedId: 'feed-2', kind: FilterRuleKind.TITLE, value: 'ad', filteredCount: 2 }
]

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

const valueInput = () => screen.getByLabelText('Value')
const typeValue = (value: string) => fireEvent.change(valueInput(), { target: { value } })
const submitButton = () => screen.getByRole('button', { name: /Add rule|Adding…/ })

describe('FeedFilterRulesCard', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUseFeeds.mockReturnValue({ data: { feeds } })
    mockUseFilterRules.mockReturnValue({ data: { rules }, error: undefined, isLoading: false })
  })

  it('lists each rule with its scope, kind, value and filtered count', () => {
    render(<FeedFilterRulesCard />)

    const [first, second, third] = screen.getAllByRole('listitem')
    expect(first).toHaveTextContent('Product announcements')
    expect(first).toHaveTextContent('Category · Claude blog · 7 filtered')
    expect(second).toHaveTextContent('sponsored')
    expect(second).toHaveTextContent('Title contains · All feeds · 0 filtered')
    expect(third).toHaveTextContent('Title contains · https://example.com/feed.xml · 2 filtered')
    expect(screen.queryByText('Deleting the rule failed.')).not.toBeInTheDocument()
    expect(screen.queryByText('No filter rules yet.')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('offers All feeds plus every feed as the scope', () => {
    render(<FeedFilterRulesCard />)
    const options = within(screen.getByLabelText('Applies to')).getAllByRole('option')
    expect(options.map((o) => o.textContent)).toEqual([
      'All feeds',
      'Claude blog',
      'https://example.com/feed.xml',
      'Email newsletter'
    ])
  })

  it('creates a rule and reports how many existing items it filtered', async () => {
    mockCreate.mockResolvedValue({ rule: { id: 'rule-4', filteredCount: 7 } })
    render(<FeedFilterRulesCard />)

    fireEvent.change(screen.getByLabelText('Applies to'), { target: { value: 'feed-1' } })
    fireEvent.change(screen.getByLabelText('Match'), {
      target: { value: String(FilterRuleKind.CATEGORY) }
    })
    typeValue('  Product announcements  ')
    fireEvent.click(submitButton())

    expect(mockCreate).toHaveBeenCalledWith({
      feedId: 'feed-1',
      kind: FilterRuleKind.CATEGORY,
      value: 'Product announcements'
    })
    const status = await screen.findByRole('status')
    expect(status).toHaveTextContent('Filtered 7 existing items.')
    expect(status).toHaveClass('text-success')
    expect(valueInput()).toHaveValue('')
  })

  it('defaults to a title rule on all feeds and says 1 item in the singular', async () => {
    mockCreate.mockResolvedValue({ rule: { id: 'rule-4', filteredCount: 1 } })
    render(<FeedFilterRulesCard />)

    typeValue('webinar')
    fireEvent.click(submitButton())

    expect(mockCreate).toHaveBeenCalledWith({
      feedId: '',
      kind: FilterRuleKind.TITLE,
      value: 'webinar'
    })
    expect(await screen.findByText('Filtered 1 existing item.')).toBeInTheDocument()
  })

  it('says so when a new rule matched no existing items', async () => {
    mockCreate.mockResolvedValue({})
    render(<FeedFilterRulesCard />)

    typeValue('rare')
    fireEvent.click(submitButton())

    expect(await screen.findByText('Rule added. No existing items matched.')).toBeInTheDocument()
  })

  it('shows Adding… and ignores resubmits while a create is pending', async () => {
    const pending = deferred<object>()
    mockCreate.mockReturnValue(pending.promise)
    render(<FeedFilterRulesCard />)

    typeValue('webinar')
    fireEvent.click(submitButton())
    expect(submitButton()).toHaveTextContent('Adding…')
    expect(submitButton()).toBeDisabled()
    fireEvent.submit(valueInput().closest('form')!)
    expect(mockCreate).toHaveBeenCalledTimes(1)

    await act(async () => pending.resolve({ rule: { filteredCount: 1 } }))
    typeValue('next')
    expect(submitButton()).toHaveTextContent('Add rule')
    expect(submitButton()).toBeEnabled()
  })

  it('starts empty and never submits a blank value', () => {
    render(<FeedFilterRulesCard />)
    expect(valueInput()).toHaveValue('')
    typeValue('   ')
    expect(submitButton()).toBeDisabled()
    const form = valueInput().closest('form')!
    const submit = createEvent.submit(form)
    fireEvent(form, submit)
    expect(submit.defaultPrevented).toBe(true)
    expect(mockCreate).not.toHaveBeenCalled()
  })

  it('explains a duplicate rule as an alert and clears it on the next attempt', async () => {
    mockCreate.mockRejectedValueOnce(new ConnectError('dup', Code.AlreadyExists))
    render(<FeedFilterRulesCard />)

    typeValue('sponsored')
    fireEvent.click(submitButton())

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('That rule already exists.')
    expect(alert).toHaveClass('text-danger')
    expect(submitButton()).toBeEnabled()

    mockCreate.mockReturnValue(new Promise(() => {}))
    fireEvent.click(submitButton())
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it.each([
    ['a non-duplicate ConnectError', new ConnectError('boom', Code.Internal)],
    ['a plain error', Object.assign(new Error('boom'), { code: Code.AlreadyExists })]
  ])('reports %s generically', async (_, err) => {
    mockCreate.mockRejectedValue(err)
    render(<FeedFilterRulesCard />)

    typeValue('x')
    fireEvent.click(submitButton())

    expect(await screen.findByText('Adding the rule failed. Please try again.')).toBeInTheDocument()
  })

  it('disables a rule while it is deleted', async () => {
    const pending = deferred<void>()
    mockDelete.mockReturnValue(pending.promise)
    render(<FeedFilterRulesCard />)

    const button = screen.getByRole('button', {
      name: 'Delete rule Category: Product announcements'
    })
    fireEvent.click(button)
    expect(mockDelete).toHaveBeenCalledWith('rule-1')
    expect(button).toBeDisabled()
    await act(async () => pending.resolve())
  })

  it('reports a failed delete, re-enables it, and clears the error on retry', async () => {
    mockDelete.mockRejectedValueOnce(new Error('boom'))
    render(<FeedFilterRulesCard />)

    const button = screen.getByRole('button', { name: 'Delete rule Title contains: sponsored' })
    fireEvent.click(button)
    expect(await screen.findByText('Deleting the rule failed.')).toBeInTheDocument()
    expect(button).toBeEnabled()

    mockDelete.mockReturnValue(new Promise(() => {}))
    fireEvent.click(button)
    await waitFor(() =>
      expect(screen.queryByText('Deleting the rule failed.')).not.toBeInTheDocument()
    )
  })

  it('shows an empty state without a list', () => {
    mockUseFilterRules.mockReturnValue({ data: { rules: [] }, error: undefined, isLoading: false })
    render(<FeedFilterRulesCard />)
    expect(screen.getByText('No filter rules yet.')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('shows loading and error states without an empty state or list', () => {
    mockUseFilterRules.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    const { rerender } = render(<FeedFilterRulesCard />)
    expect(screen.getByText('Loading filter rules…')).toBeInTheDocument()
    expect(screen.queryByText('No filter rules yet.')).not.toBeInTheDocument()
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument()

    mockUseFilterRules.mockReturnValue({ data: undefined, error: new Error(), isLoading: false })
    rerender(<FeedFilterRulesCard />)
    expect(screen.getByText('Failed to load filter rules.')).toBeInTheDocument()
    expect(screen.queryByText('No filter rules yet.')).not.toBeInTheDocument()
  })

  it("names a feed rule's scope Unknown feed until feeds load", () => {
    mockUseFeeds.mockReturnValue({ data: undefined })
    render(<FeedFilterRulesCard />)
    expect(screen.getAllByRole('listitem')[0]).toHaveTextContent('Unknown feed')
    const options = within(screen.getByLabelText('Applies to')).getAllByRole('option')
    expect(options.map((o) => o.textContent)).toEqual(['All feeds'])
  })
})
