import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ManageDuplicatesDialog from '@/components/backlog/ManageDuplicatesDialog'

// ---------------------------------------------------------------------------
// Mock setup
// ---------------------------------------------------------------------------

const mockMergeBooks = jest.fn()
const mockFindDuplicatesData = {
  data: undefined as { groups: unknown[] } | undefined,
  isLoading: false,
  mutate: jest.fn()
}
const mockMutate = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useFindDuplicates: () => mockFindDuplicatesData,
  useMergeBooks: () => mockMergeBooks
}))

jest.mock('swr', () => ({
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeEntry(bookId: string, title: string, status = 'to-read', formats: string[] = []) {
  return {
    id: `ub-${bookId}`,
    bookId,
    userId: 'user1',
    book: {
      id: bookId,
      title,
      authors: ['Some Author'],
      isbn13: '',
      isbn10: '',
      coverUrl: '',
      description: '',
      pageCount: 0,
      externalRefs: {} as Record<string, string>
    },
    status,
    tags: [],
    rating: 0,
    notes: '',
    finishedAt: [],
    addedAt: '',
    updatedAt: '',
    progressMode: 'pages',
    currentPage: 0,
    progressPercent: 0,
    formats
  }
}

function makeGroup(entries: ReturnType<typeof makeEntry>[], reason = 'isbn13') {
  return { entries, reason }
}

function renderDialog(open = true, onOpenChange = jest.fn()) {
  return render(<ManageDuplicatesDialog open={open} onOpenChange={onOpenChange} />)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('ManageDuplicatesDialog', () => {
  beforeEach(() => {
    mockMergeBooks.mockReset()
    mockMutate.mockReset()
    mockFindDuplicatesData.data = undefined
    mockFindDuplicatesData.isLoading = false
    mockFindDuplicatesData.mutate = jest.fn()
  })

  it('renders the dialog title when open', () => {
    renderDialog()
    expect(screen.getByText('Find duplicates')).toBeInTheDocument()
  })

  it('shows scanning message while loading', () => {
    mockFindDuplicatesData.isLoading = true
    renderDialog()
    expect(screen.getByText(/Scanning library/)).toBeInTheDocument()
  })

  it('shows empty state when no duplicates found', () => {
    mockFindDuplicatesData.data = { groups: [] }
    renderDialog()
    expect(screen.getByText('No duplicates found.')).toBeInTheDocument()
  })

  it('renders a duplicate group with entries and radio buttons', () => {
    const entry1 = makeEntry('book-a', 'The Hobbit', 'to-read')
    const entry2 = makeEntry('book-b', 'The Hobbit (Special Ed.)', 'read')
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    expect(screen.getByText('The Hobbit')).toBeInTheDocument()
    expect(screen.getByText('The Hobbit (Special Ed.)')).toBeInTheDocument()
    // Two radio buttons — one per entry.
    const radios = screen.getAllByRole('radio')
    expect(radios).toHaveLength(2)
    // First entry (index 0) is selected by default.
    expect(radios[0]).toBeChecked()
    expect(radios[1]).not.toBeChecked()
  })

  it('shows "Keep this entry" label for the selected winner', () => {
    const entry1 = makeEntry('book-a', 'BookWinner')
    const entry2 = makeEntry('book-b', 'BookLoser')
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    const keepLabel = screen.getByText('Keep this entry')
    expect(keepLabel).toBeInTheDocument()
  })

  it('shows reason label', () => {
    const entry1 = makeEntry('book-c', 'DuneA')
    const entry2 = makeEntry('book-d', 'DuneB')
    mockFindDuplicatesData.data = {
      groups: [makeGroup([entry1, entry2], 'title+author')]
    }

    renderDialog()

    expect(screen.getByText('Same title + author')).toBeInTheDocument()
  })

  it('calls mergeBooks with correct winner and losers on Merge click', async () => {
    mockMergeBooks.mockResolvedValue({})
    const entry1 = makeEntry('winner-id', 'BookA')
    const entry2 = makeEntry('loser-id', 'BookB')
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    const mergeBtn = screen.getByRole('button', { name: 'Merge' })
    fireEvent.click(mergeBtn)

    await waitFor(() => expect(mockMergeBooks).toHaveBeenCalledTimes(1))
    expect(mockMergeBooks).toHaveBeenCalledWith('winner-id', ['loser-id'])
    expect(mockMutate).toHaveBeenCalledWith('/backlog/books')
  })

  it('switches winner when a different radio is selected', async () => {
    mockMergeBooks.mockResolvedValue({})
    const entry1 = makeEntry('book-x', 'OriginalWinner')
    const entry2 = makeEntry('book-y', 'NewWinner')
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    // Select the second entry as winner.
    const radios = screen.getAllByRole('radio')
    fireEvent.click(radios[1])

    const mergeBtn = screen.getByRole('button', { name: 'Merge' })
    fireEvent.click(mergeBtn)

    await waitFor(() => expect(mockMergeBooks).toHaveBeenCalledTimes(1))
    // Winner is now book-y; loser is book-x.
    expect(mockMergeBooks).toHaveBeenCalledWith('book-y', ['book-x'])
  })

  it('shows error when merge fails', async () => {
    mockMergeBooks.mockRejectedValue(new Error('server error'))
    const entry1 = makeEntry('ea', 'BookErrA')
    const entry2 = makeEntry('eb', 'BookErrB')
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    fireEvent.click(screen.getByRole('button', { name: 'Merge' }))

    await waitFor(() => expect(screen.getByText(/Merge failed/)).toBeInTheDocument())
  })

  it('shows Merge all button when more than one group', () => {
    const g1 = makeGroup([makeEntry('ga1', 'G1A'), makeEntry('ga2', 'G1B')])
    const g2 = makeGroup([makeEntry('gb1', 'G2A'), makeEntry('gb2', 'G2B')])
    mockFindDuplicatesData.data = { groups: [g1, g2] }

    renderDialog()

    expect(screen.getByRole('button', { name: /Merge all/ })).toBeInTheDocument()
  })

  it('renders title-initials placeholder when an entry has no cover URL', () => {
    // makeEntry sets coverUrl to '' — BookCover should fall back to initials.
    const entry1 = makeEntry('book-nc', 'Dark Matter')
    const entry2 = makeEntry('book-nc2', 'Dark Matter (Alt)')
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    // BookCover renders initials("Dark Matter") → "DM"
    expect(screen.getAllByText('DM').length).toBeGreaterThan(0)
  })

  it('does not show Merge all button for a single group', () => {
    const g1 = makeGroup([makeEntry('s1', 'SingleA'), makeEntry('s2', 'SingleB')])
    mockFindDuplicatesData.data = { groups: [g1] }

    renderDialog()

    expect(screen.queryByRole('button', { name: /Merge all/ })).not.toBeInTheDocument()
  })

  it('calls mergeBooks for each group on Merge all click', async () => {
    mockMergeBooks.mockResolvedValue({})
    const g1 = makeGroup([makeEntry('w1', 'G1W'), makeEntry('l1', 'G1L')])
    const g2 = makeGroup([makeEntry('w2', 'G2W'), makeEntry('l2', 'G2L')])
    mockFindDuplicatesData.data = { groups: [g1, g2] }

    renderDialog()

    fireEvent.click(screen.getByRole('button', { name: /Merge all/ }))

    await waitFor(() => expect(mockMergeBooks).toHaveBeenCalledTimes(2))
    expect(mockMergeBooks).toHaveBeenNthCalledWith(1, 'w1', ['l1'])
    expect(mockMergeBooks).toHaveBeenNthCalledWith(2, 'w2', ['l2'])
  })
})
