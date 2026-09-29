import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { ConnectError, Code } from '@connectrpc/connect'
import FeedRuleSuggestionsCard from '@/components/feeds/FeedRuleSuggestionsCard'
import { FilterRuleKind } from '@/lib/gen/feeds/v1/feeds_pb'

const mockUseSuggestions = jest.fn()
const mockCreate = jest.fn()
const mockDismiss = jest.fn()
jest.mock('@/hooks/useFeedRuleSuggestions', () => ({
  useFilterRuleSuggestions: () => mockUseSuggestions(),
  useDismissFilterRuleSuggestion: () => mockDismiss
}))
jest.mock('@/hooks/useFeeds', () => ({
  useCreateFilterRule: () => mockCreate
}))

const suggestions = [
  {
    feedId: 'feed-1',
    feedTitle: 'Claude blog',
    feedUrl: 'https://claude.com/blog',
    category: 'Product announcements',
    itemCount: 14,
    readCount: 0
  },
  {
    feedId: 'feed-2',
    feedTitle: '',
    feedUrl: 'https://example.com/feed.xml',
    category: 'Sponsored',
    itemCount: 20,
    readCount: 2
  }
]

const rows = () => screen.queryAllByRole('listitem')
const rowButton = (index: number, name: string) =>
  within(rows()[index]!).getByRole('button', { name })

describe('FeedRuleSuggestionsCard', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUseSuggestions.mockReturnValue({ data: { suggestions } })
  })

  it('renders nothing when there are no suggestions', () => {
    mockUseSuggestions.mockReturnValue({ data: { suggestions: [] } })
    const { container } = render(<FeedRuleSuggestionsCard />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing before the suggestions load', () => {
    mockUseSuggestions.mockReturnValue({ data: undefined })
    const { container } = render(<FeedRuleSuggestionsCard />)
    expect(container).toBeEmptyDOMElement()
  })

  it('lists each suggestion with its feed, category and read count', () => {
    render(<FeedRuleSuggestionsCard />)
    expect(screen.getByText('Suggested filters')).toBeInTheDocument()
    const [first, second] = rows()
    expect(first).toHaveTextContent('Claude blog · Product announcements · read 0 of 14')
    expect(second).toHaveTextContent('https://example.com/feed.xml · Sponsored · read 2 of 20')
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it("creates the feed's category rule and reports how many items it filtered", async () => {
    mockCreate.mockResolvedValue({ rule: { filteredCount: 14 } })
    render(<FeedRuleSuggestionsCard />)

    fireEvent.click(rowButton(0, 'Create rule'))

    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('Filtered 14 existing items.')
    )
    expect(mockCreate).toHaveBeenCalledWith({
      feedId: 'feed-1',
      kind: FilterRuleKind.CATEGORY,
      value: 'Product announcements'
    })
  })

  it('keeps the result visible once the last suggestion is gone', async () => {
    mockCreate.mockResolvedValue({ rule: { filteredCount: 1 } })
    const { rerender } = render(<FeedRuleSuggestionsCard />)
    fireEvent.click(rowButton(0, 'Create rule'))
    await screen.findByText('Filtered 1 existing item.')

    mockUseSuggestions.mockReturnValue({ data: { suggestions: [] } })
    rerender(<FeedRuleSuggestionsCard />)

    expect(screen.getByRole('status')).toHaveTextContent('Filtered 1 existing item.')
    expect(screen.getByRole('status')).toHaveClass('text-success')
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('reports a rule that filtered nothing', async () => {
    mockCreate.mockResolvedValue({})
    render(<FeedRuleSuggestionsCard />)

    fireEvent.click(rowButton(0, 'Create rule'))

    expect(await screen.findByRole('status')).toHaveTextContent(
      'Rule added. No existing items matched.'
    )
  })

  it('shows the row as adding while its rule is created', async () => {
    let resolve!: (v: unknown) => void
    mockCreate.mockReturnValue(new Promise((res) => (resolve = res)))
    render(<FeedRuleSuggestionsCard />)

    fireEvent.click(rowButton(0, 'Create rule'))

    expect(rowButton(0, 'Adding…')).toBeDisabled()
    expect(rowButton(0, 'Dismiss')).toBeDisabled()
    resolve({ rule: { filteredCount: 2 } })
    await waitFor(() => expect(rowButton(0, 'Create rule')).not.toBeDisabled())
  })

  it('clears the previous result when another action starts', async () => {
    mockCreate.mockRejectedValue(new Error('boom'))
    mockDismiss.mockReturnValue(new Promise(() => {}))
    render(<FeedRuleSuggestionsCard />)
    fireEvent.click(rowButton(0, 'Create rule'))
    await screen.findByRole('alert')

    fireEvent.click(rowButton(1, 'Dismiss'))

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('clears the previous result when creating another rule', async () => {
    mockCreate.mockRejectedValueOnce(new Error('boom')).mockReturnValue(new Promise(() => {}))
    render(<FeedRuleSuggestionsCard />)
    fireEvent.click(rowButton(0, 'Create rule'))
    await screen.findByRole('alert')

    fireEvent.click(rowButton(1, 'Create rule'))

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it("keeps a row's pending state with its suggestion when the list reorders", () => {
    mockDismiss.mockReturnValue(new Promise(() => {}))
    const { rerender } = render(<FeedRuleSuggestionsCard />)
    fireEvent.click(rowButton(1, 'Dismiss'))

    mockUseSuggestions.mockReturnValue({ data: { suggestions: [...suggestions].reverse() } })
    rerender(<FeedRuleSuggestionsCard />)

    expect(rows()[0]).toHaveTextContent('Sponsored')
    expect(rowButton(0, 'Dismissing…')).toBeDisabled()
    expect(rowButton(1, 'Dismiss')).not.toBeDisabled()
  })

  it('reports a rule that already exists', async () => {
    mockCreate.mockRejectedValue(new ConnectError('dup', Code.AlreadyExists))
    render(<FeedRuleSuggestionsCard />)

    fireEvent.click(rowButton(1, 'Create rule'))

    expect(await screen.findByRole('alert')).toHaveTextContent('That rule already exists.')
    expect(rowButton(1, 'Create rule')).not.toBeDisabled()
  })

  it('reports any other failure to create the rule', async () => {
    mockCreate.mockRejectedValue(new Error('boom'))
    render(<FeedRuleSuggestionsCard />)

    fireEvent.click(rowButton(0, 'Create rule'))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Adding the rule failed. Please try again.'
    )
  })

  it('disables both actions while one is pending', async () => {
    let resolve!: () => void
    mockDismiss.mockReturnValue(new Promise<void>((res) => (resolve = res)))
    render(<FeedRuleSuggestionsCard />)

    fireEvent.click(rowButton(0, 'Dismiss'))

    expect(rowButton(0, 'Dismissing…')).toBeDisabled()
    expect(rowButton(0, 'Create rule')).toBeDisabled()
    expect(rowButton(1, 'Dismiss')).not.toBeDisabled()
    resolve()
    await waitFor(() => expect(mockDismiss).toHaveBeenCalledWith('feed-1', 'Product announcements'))
  })

  it('reports a failed dismissal and re-enables the row', async () => {
    mockDismiss.mockRejectedValue(new Error('boom'))
    render(<FeedRuleSuggestionsCard />)

    fireEvent.click(rowButton(1, 'Dismiss'))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Dismissing the suggestion failed. Please try again.'
    )
    expect(rowButton(1, 'Dismiss')).not.toBeDisabled()
  })
})
