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

function makeEntry(
  bookId: string,
  title: string,
  overrides: Partial<{
    status: string
    formats: string[]
    isbn13: string
    coverUrl: string
    description: string
    pageCount: number
    authors: string[]
  }> = {}
) {
  return {
    id: `ub-${bookId}`,
    bookId,
    userId: 'user1',
    book: {
      id: bookId,
      title,
      authors: overrides.authors ?? ['Some Author'],
      isbn13: overrides.isbn13 ?? '',
      coverUrl: overrides.coverUrl ?? '',
      description: overrides.description ?? '',
      pageCount: overrides.pageCount ?? 0
    },
    status: overrides.status ?? 'to-read',
    tags: [],
    rating: 0,
    notes: '',
    finishedAt: [],
    addedAt: '',
    updatedAt: '',
    progressMode: 'pages',
    currentPage: 0,
    progressPercent: 0,
    formats: overrides.formats ?? []
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
    const entry1 = makeEntry('book-a', 'The Hobbit')
    const entry2 = makeEntry('book-b', 'The Hobbit (Special Ed.)')
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    // Both entry titles appear; each title also appears in the conflict-picker
    // chips (titles differ across entries), so use getAllByText.
    expect(screen.getAllByText('The Hobbit').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('The Hobbit (Special Ed.)').length).toBeGreaterThanOrEqual(1)
    // At least two entry-winner radios are present (conflict pickers may add more).
    const radios = screen.getAllByRole('radio')
    expect(radios.length).toBeGreaterThanOrEqual(2)
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

  it('calls mergeBooks with winner, losers, and options on Merge click (no conflicts)', async () => {
    mockMergeBooks.mockResolvedValue({})
    // Both entries have identical book fields — no conflicts detected.
    const entry1 = makeEntry('winner-id', 'BookA')
    const entry2 = makeEntry('loser-id', 'BookA')
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    const mergeBtn = screen.getByRole('button', { name: 'Merge' })
    fireEvent.click(mergeBtn)

    await waitFor(() => expect(mockMergeBooks).toHaveBeenCalledTimes(1))
    // Third arg is always the options object; cover source omitted when no cover conflict.
    expect(mockMergeBooks).toHaveBeenCalledWith(
      'winner-id',
      ['loser-id'],
      expect.objectContaining({ resolvedCoverSourceBookId: undefined })
    )
    expect(mockMutate).toHaveBeenCalledWith('/backlog/books')
  })

  it('switches winner when a different radio is selected', async () => {
    mockMergeBooks.mockResolvedValue({})
    const entry1 = makeEntry('book-x', 'OriginalWinner')
    const entry2 = makeEntry('book-y', 'NewWinner')
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    // Select the second entry's winner radio (first radio group).
    const winnerRadios = screen
      .getAllByRole('radio')
      .filter((r) => r instanceof HTMLInputElement && r.name.startsWith('winner-'))
    fireEvent.click(winnerRadios[1])

    const mergeBtn = screen.getByRole('button', { name: 'Merge' })
    fireEvent.click(mergeBtn)

    await waitFor(() => expect(mockMergeBooks).toHaveBeenCalledTimes(1))
    // Winner is now book-y; loser is book-x.
    expect(mockMergeBooks).toHaveBeenCalledWith('book-y', ['book-x'], expect.any(Object))
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
    expect(mockMergeBooks).toHaveBeenNthCalledWith(1, 'w1', ['l1'], expect.any(Object))
    expect(mockMergeBooks).toHaveBeenNthCalledWith(2, 'w2', ['l2'], expect.any(Object))
  })

  it('shows conflict field picker when entries have differing page counts', () => {
    const entry1 = makeEntry('book-p1', 'SameTitle', { pageCount: 320 })
    const entry2 = makeEntry('book-p2', 'SameTitle', { pageCount: 310 })
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    // Conflict section header should appear
    const conflictSection = screen.getByText(/Resolve.*conflicting/)
    expect(conflictSection).toBeInTheDocument()
    // "Page count" appears as a quality badge in each entry and as a conflict
    // field label — assert at least one exists.
    expect(screen.getAllByText('Page count').length).toBeGreaterThanOrEqual(1)
  })

  it('does not show conflict picker when all fields agree', () => {
    const entry1 = makeEntry('book-same1', 'Same Book', {
      pageCount: 300,
      isbn13: '9781234567890'
    })
    const entry2 = makeEntry('book-same2', 'Same Book', {
      pageCount: 300,
      isbn13: '9781234567890'
    })
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    expect(screen.queryByText(/Resolve.*conflicting/)).not.toBeInTheDocument()
  })

  it('detects a status conflict and shows Shelf / status picker', () => {
    // Both entries have the same book fields but different statuses.
    const entry1 = makeEntry('status-a', 'SharedTitle', { status: 'sci-fi' })
    const entry2 = makeEntry('status-b', 'SharedTitle', { status: 'read' })
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    expect(screen.getByText('Shelf / status')).toBeInTheDocument()
    expect(screen.getAllByText('sci-fi').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('read').length).toBeGreaterThanOrEqual(1)
  })

  it('sends resolvedStatus when entries differ in status', async () => {
    mockMergeBooks.mockResolvedValue({})
    // winner (entries[0]) is on 'to-read'; loser is on a custom shelf 'sci-fi'
    const entry1 = makeEntry('winner-s', 'SameTitle', { status: 'to-read' })
    const entry2 = makeEntry('loser-s', 'SameTitle', { status: 'sci-fi' })
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    // The auto-status default should have pre-selected sci-fi (loser-s) since
    // custom shelf outranks to-read. Click Merge without changing the picker.
    const mergeBtn = screen.getByRole('button', { name: 'Merge' })
    fireEvent.click(mergeBtn)

    await waitFor(() => expect(mockMergeBooks).toHaveBeenCalledTimes(1))
    expect(mockMergeBooks).toHaveBeenCalledWith(
      'winner-s',
      ['loser-s'],
      expect.objectContaining({ resolvedStatus: 'sci-fi' })
    )
  })

  it('overrides resolvedStatus when user picks a different entry from status picker', async () => {
    mockMergeBooks.mockResolvedValue({})
    const entry1 = makeEntry('w-ov', 'SameTitle', { status: 'sci-fi' })
    const entry2 = makeEntry('l-ov', 'SameTitle', { status: 'fantasy' })
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    // Both are custom shelves — auto picks entries[0] ('sci-fi').
    // User explicitly picks entries[1] ('fantasy') via the status radio.
    const statusRadios = screen
      .getAllByRole('radio')
      .filter((r) => r instanceof HTMLInputElement && r.name.startsWith('status-'))
    // statusRadios[1] corresponds to the second entry (l-ov / fantasy)
    fireEvent.click(statusRadios[1])

    fireEvent.click(screen.getByRole('button', { name: 'Merge' }))

    await waitFor(() => expect(mockMergeBooks).toHaveBeenCalledTimes(1))
    expect(mockMergeBooks).toHaveBeenCalledWith(
      'w-ov',
      ['l-ov'],
      expect.objectContaining({ resolvedStatus: 'fantasy' })
    )
  })

  it('passes resolvedCoverSourceBookId when entries have differing cover presence', async () => {
    mockMergeBooks.mockResolvedValue({})
    // winner has no cover, loser has one
    const entry1 = makeEntry('cov-winner', 'CoverBook', { coverUrl: '' })
    const entry2 = makeEntry('cov-loser', 'CoverBook', {
      coverUrl: 'https://example.com/cover.jpg'
    })
    mockFindDuplicatesData.data = { groups: [makeGroup([entry1, entry2])] }

    renderDialog()

    // Cover conflict picker is shown; loser entry's "Use this" radio for cover
    const coverRadios = screen
      .getAllByRole('radio')
      .filter((r) => r instanceof HTMLInputElement && r.name.startsWith('cover-'))
    expect(coverRadios.length).toBeGreaterThanOrEqual(2)

    // Select the loser's cover
    fireEvent.click(coverRadios[1])

    const mergeBtn = screen.getByRole('button', { name: 'Merge' })
    fireEvent.click(mergeBtn)

    await waitFor(() => expect(mockMergeBooks).toHaveBeenCalledTimes(1))
    expect(mockMergeBooks).toHaveBeenCalledWith(
      'cov-winner',
      ['cov-loser'],
      expect.objectContaining({ resolvedCoverSourceBookId: 'cov-loser' })
    )
  })
})
