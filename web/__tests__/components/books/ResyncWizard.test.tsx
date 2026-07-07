import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ResyncWizard from '@/components/books/ResyncWizard'

const mockApplyResyncChoice = jest.fn()
const mockProposalsData: {
  data: { proposals: unknown[] } | undefined
  isLoading: boolean
} = {
  data: undefined,
  isLoading: false
}
const mockMutate = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useResyncProposals: () => mockProposalsData,
  useApplyResyncChoice: () => mockApplyResyncChoice
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
        source: 'openlibrary',
        title: `${title} (OL)`,
        authors: ['Some Author'],
        coverUrl: '',
        description: '',
        pageCount: 0,
        isbn13: '',
        differs: ['title']
      }
    ]
  }
}

beforeEach(() => {
  mockApplyResyncChoice.mockReset().mockResolvedValue({})
  mockMutate.mockReset()
  mockProposalsData.data = undefined
  mockProposalsData.isLoading = false
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
    // "Open Library" appears both as the source card label and the radio option.
    expect(screen.getAllByText('Open Library').length).toBeGreaterThanOrEqual(2)
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

    fireEvent.click(screen.getByRole('radio', { name: 'Open Library' }))
    fireEvent.click(screen.getByRole('button', { name: /apply & next/i }))

    await waitFor(() => {
      expect(mockApplyResyncChoice).toHaveBeenCalledWith('b1', 'openlibrary')
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
      proposals: [
        {
          bookId: 'b1',
          library: {
            source: '',
            title: 'Obscure Book',
            authors: [],
            coverUrl: '',
            description: '',
            pageCount: 0,
            isbn13: '',
            differs: []
          },
          sources: []
        }
      ]
    }
    render(<ResyncWizard />)

    expect(screen.getByText('Not found in any source')).toBeInTheDocument()
    expect(screen.getByText(/consider adding a new source/i)).toBeInTheDocument()
    // Only "Keep library" is offered — there's no source to pick from.
    expect(screen.getAllByRole('radio')).toHaveLength(1)
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
