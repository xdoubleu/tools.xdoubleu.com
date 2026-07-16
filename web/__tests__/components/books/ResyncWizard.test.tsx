import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ResyncWizard from '@/components/books/ResyncWizard'

const mockApplyResyncChoice = jest.fn()
const mockApplyBookSource = jest.fn()
const mockProposalsData: {
  data: { proposals: unknown[] } | undefined
  isLoading: boolean
} = {
  data: undefined,
  isLoading: false
}
const mockSourcesData: {
  data: { proposal: unknown } | undefined
  isLoading: boolean
  error: Error | undefined
} = {
  data: undefined,
  isLoading: false,
  error: undefined
}
const mockUseBookSources: jest.Mock<typeof mockSourcesData, unknown[]> = jest.fn(
  () => mockSourcesData
)
const mockMutate = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useResyncProposals: () => mockProposalsData,
  useApplyResyncChoice: () => mockApplyResyncChoice,
  useBookSources: (...args: unknown[]) => mockUseBookSources(...args),
  useApplyBookSource: () => mockApplyBookSource
}))

jest.mock('swr', () => ({
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

function makeProposal(bookId: string, title: string) {
  return {
    bookId,
    library: {
      source: '',
      title,
      authors: ['Some Author'],
      coverUrl: '',
      description: '',
      pageCount: 0,
      isbn13: '',
      differs: []
    },
    sources: [
      {
        source: 'unicat',
        title: `${title} (UC)`,
        authors: ['Some Author'],
        coverUrl: '',
        description: '',
        pageCount: 0,
        isbn13: '',
        differs: ['title'],
        index: 0
      }
    ]
  }
}

function makeNotFoundProposal(bookId: string, title: string) {
  return {
    bookId,
    library: {
      source: '',
      title,
      authors: [],
      coverUrl: '',
      description: '',
      pageCount: 0,
      isbn13: '',
      differs: []
    },
    sources: []
  }
}

beforeEach(() => {
  mockApplyResyncChoice.mockReset().mockResolvedValue({})
  mockApplyBookSource.mockReset().mockResolvedValue({})
  mockUseBookSources.mockClear()
  mockMutate.mockReset()
  mockProposalsData.data = undefined
  mockProposalsData.isLoading = false
  mockSourcesData.data = undefined
  mockSourcesData.isLoading = false
  mockSourcesData.error = undefined
})

describe('ResyncWizard', () => {
  it('shows an empty state when there are no flagged proposals', () => {
    mockProposalsData.data = { proposals: [] }
    render(<ResyncWizard />)
    expect(screen.getByText(/no flagged differences/i)).toBeInTheDocument()
  })

  it('shows a loading state', () => {
    mockProposalsData.isLoading = true
    render(<ResyncWizard />)
    expect(screen.getByText(/loading/i)).toBeInTheDocument()
  })

  it('renders the current book title and each source card', () => {
    mockProposalsData.data = { proposals: [makeProposal('b1', 'Dune')] }
    render(<ResyncWizard />)
    // "Dune" appears both as the page heading and as the Library card's title
    // field value.
    expect(screen.getAllByText('Dune').length).toBeGreaterThanOrEqual(2)
    expect(screen.getByText('Library')).toBeInTheDocument()
    // "UniCat" appears both as the source card label and the radio option.
    expect(screen.getAllByText('UniCat').length).toBeGreaterThanOrEqual(2)
    expect(screen.getByText('Book 1 of 1')).toBeInTheDocument()
  })

  it('dismisses (keeps library) without a source selected', async () => {
    mockProposalsData.data = { proposals: [makeProposal('b1', 'Dune')] }
    render(<ResyncWizard />)

    fireEvent.click(screen.getByRole('button', { name: /dismiss/i }))

    await waitFor(() => {
      expect(mockApplyResyncChoice).toHaveBeenCalledWith('b1', '')
    })
    expect(mockMutate).toHaveBeenCalledWith('/books/resync-proposals')
  })

  it('applies the chosen source', async () => {
    mockProposalsData.data = { proposals: [makeProposal('b1', 'Dune')] }
    render(<ResyncWizard />)

    fireEvent.click(screen.getByRole('radio', { name: 'UniCat' }))
    fireEvent.click(screen.getByRole('button', { name: /apply & next/i }))

    await waitFor(() => {
      expect(mockApplyResyncChoice).toHaveBeenCalledWith('b1', 'unicat')
    })
  })

  it('steps between books with Prev/Next', () => {
    mockProposalsData.data = {
      proposals: [makeProposal('b1', 'Dune'), makeProposal('b2', 'Emma')]
    }
    render(<ResyncWizard />)

    expect(screen.getByText('Book 1 of 2')).toBeInTheDocument()
    expect(screen.getAllByText('Dune').length).toBeGreaterThanOrEqual(1)
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('Book 2 of 2')).toBeInTheDocument()
    expect(screen.getAllByText('Emma').length).toBeGreaterThanOrEqual(1)
    fireEvent.click(screen.getByRole('button', { name: 'Prev' }))
    expect(screen.getByText('Book 1 of 2')).toBeInTheDocument()
  })

  it('shows a distinct state for a book not found in any source', () => {
    mockProposalsData.data = {
      proposals: [makeNotFoundProposal('b1', 'Obscure Book')]
    }
    render(<ResyncWizard />)

    expect(screen.getByText('Not found in any source')).toBeInTheDocument()
    expect(screen.getByText(/consider adding a new source/i)).toBeInTheDocument()
    // Only "Keep library" is offered — there's no source to pick from.
    expect(screen.getAllByRole('radio')).toHaveLength(1)
  })

  it('filters to books not found in any source', () => {
    mockProposalsData.data = {
      proposals: [makeProposal('b1', 'Dune'), makeNotFoundProposal('b2', 'Obscure Book')]
    }
    render(<ResyncWizard />)

    expect(screen.getByText('Book 1 of 2')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Not found only (1)' }))
    expect(screen.getByText('Book 1 of 1')).toBeInTheDocument()
    expect(screen.getAllByText('Obscure Book').length).toBeGreaterThanOrEqual(1)
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()

    // Toggling off restores the full list.
    fireEvent.click(screen.getByRole('button', { name: 'Not found only (1)' }))
    expect(screen.getByText('Book 1 of 2')).toBeInTheDocument()
  })

  it('shows an empty state when the filter matches nothing', () => {
    mockProposalsData.data = { proposals: [makeProposal('b1', 'Dune')] }
    render(<ResyncWizard />)

    fireEvent.click(screen.getByRole('button', { name: 'Not found only (0)' }))
    expect(screen.getByText(/every source found every flagged book/i)).toBeInTheDocument()
  })

  it('re-runs the search with tweaked terms and applies via the override path', async () => {
    mockProposalsData.data = {
      proposals: [makeNotFoundProposal('b1', 'Misspelled Titel')]
    }
    mockSourcesData.data = { proposal: makeProposal('b1', 'Correct Title') }
    render(<ResyncWizard />)

    // The live fetch stays disabled until a tweaked search is submitted.
    expect(mockUseBookSources).toHaveBeenLastCalledWith('b1', false, undefined)

    fireEvent.change(screen.getByPlaceholderText('Title'), {
      target: { value: 'Correct Title' }
    })
    fireEvent.click(screen.getByRole('button', { name: /search with these terms/i }))

    expect(mockUseBookSources).toHaveBeenLastCalledWith('b1', true, {
      title: 'Correct Title',
      author: ''
    })
    // The live proposal replaces the stored one.
    expect(screen.getAllByText('Correct Title').length).toBeGreaterThanOrEqual(1)

    fireEvent.click(screen.getByRole('radio', { name: 'UniCat' }))
    fireEvent.click(screen.getByRole('button', { name: /apply & next/i }))

    await waitFor(() => {
      expect(mockApplyBookSource).toHaveBeenCalledWith('b1', 'unicat', 0, {
        title: 'Correct Title',
        author: ''
      })
    })
    expect(mockApplyResyncChoice).not.toHaveBeenCalled()
    expect(mockMutate).toHaveBeenCalledWith('/books/resync-proposals')
  })

  it('shows an error message when applying fails', async () => {
    mockApplyResyncChoice.mockRejectedValueOnce(new Error('boom'))
    mockProposalsData.data = { proposals: [makeProposal('b1', 'Dune')] }
    render(<ResyncWizard />)

    fireEvent.click(screen.getByRole('button', { name: /dismiss/i }))

    await waitFor(() => {
      expect(screen.getByText('boom')).toBeInTheDocument()
    })
  })
})
