import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { ResyncProposalSchema, SourceBookSchema } from '@/lib/gen/books/v1/catalog_pb'
import SourceCompare from '@/components/books/SourceCompare'

function makeProposal() {
  return create(ResyncProposalSchema, {
    bookId: 'b1',
    library: create(SourceBookSchema, {
      source: '',
      title: 'Dune',
      authors: ['Frank Herbert']
    }),
    sources: [
      create(SourceBookSchema, {
        source: 'openlibrary',
        title: 'Dune (OL)',
        coverUrl: 'https://example.com/cover.jpg',
        differs: ['title', 'authors']
      })
    ]
  })
}

describe('SourceCompare', () => {
  it('renders a cover image for a source that has one', () => {
    render(
      <SourceCompare proposal={makeProposal()} onApply={jest.fn()} applyLabel={() => 'Apply'} />
    )
    const img = screen.getByRole('img')
    expect(img).toHaveAttribute('alt', 'Dune (OL)')
  })

  it('does not render cover_url as a text field', () => {
    render(
      <SourceCompare proposal={makeProposal()} onApply={jest.fn()} applyLabel={() => 'Apply'} />
    )
    expect(screen.queryByText('cover url')).not.toBeInTheDocument()
  })

  it('calls onApply with the selected source and index 0', async () => {
    const onApply = jest.fn().mockResolvedValue(undefined)
    render(<SourceCompare proposal={makeProposal()} onApply={onApply} applyLabel={() => 'Apply'} />)

    fireEvent.click(screen.getByRole('radio', { name: 'Open Library' }))
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() => expect(onApply).toHaveBeenCalledWith('openlibrary', 0))
  })

  it('shows an error message when onApply rejects', async () => {
    const onApply = jest.fn().mockRejectedValue(new Error('boom'))
    render(<SourceCompare proposal={makeProposal()} onApply={onApply} applyLabel={() => 'Apply'} />)

    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() => expect(screen.getByText('boom')).toBeInTheDocument())
  })

  it('uses applyLabel to derive the button text from the current choice', () => {
    render(
      <SourceCompare
        proposal={makeProposal()}
        onApply={jest.fn()}
        applyLabel={(choice) => (choice === '' ? 'Dismiss' : 'Apply & next')}
      />
    )
    expect(screen.getByRole('button', { name: 'Dismiss' })).toBeInTheDocument()
  })

  it('renders no search fields without onSearch', () => {
    render(
      <SourceCompare proposal={makeProposal()} onApply={jest.fn()} applyLabel={() => 'Apply'} />
    )
    expect(screen.queryByPlaceholderText('Title')).not.toBeInTheDocument()
  })

  it('prefills the search fields from the library row and submits edited terms', () => {
    const onSearch = jest.fn()
    render(
      <SourceCompare
        proposal={makeProposal()}
        onApply={jest.fn()}
        applyLabel={() => 'Apply'}
        onSearch={onSearch}
      />
    )

    const title = screen.getByPlaceholderText('Title')
    const author = screen.getByPlaceholderText('Author')
    expect(title).toHaveValue('Dune')
    expect(author).toHaveValue('Frank Herbert')

    fireEvent.change(title, { target: { value: 'Dune Messiah' } })
    fireEvent.click(screen.getByRole('button', { name: /search with these terms/i }))

    expect(onSearch).toHaveBeenCalledWith('Dune Messiah', 'Frank Herbert')
  })

  it('calls onApply with index 0 when dismissing (keep library)', async () => {
    const onApply = jest.fn().mockResolvedValue(undefined)
    render(
      <SourceCompare
        proposal={makeProposal()}
        onApply={onApply}
        applyLabel={(choice) => (choice === '' ? 'Dismiss' : 'Apply')}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))

    await waitFor(() => expect(onApply).toHaveBeenCalledWith('', 0))
  })

  it('renders every candidate when a source has multiple (override search)', () => {
    const proposal = create(ResyncProposalSchema, {
      bookId: 'b1',
      library: create(SourceBookSchema, { source: '', title: 'Dune' }),
      sources: [
        create(SourceBookSchema, { source: 'hardcover', title: 'Candidate A', index: 0 }),
        create(SourceBookSchema, { source: 'hardcover', title: 'Candidate B', index: 1 }),
        create(SourceBookSchema, { source: 'hardcover', title: 'Candidate C', index: 2 })
      ]
    })
    render(<SourceCompare proposal={proposal} onApply={jest.fn()} applyLabel={() => 'Apply'} />)

    expect(screen.getByText('Candidate A')).toBeInTheDocument()
    expect(screen.getByText('Candidate B')).toBeInTheDocument()
    expect(screen.getByText('Candidate C')).toBeInTheDocument()
    expect(screen.getByText('Hardcover (3 candidates)')).toBeInTheDocument()
  })

  it('applies the chosen candidate index, not just the first, for a multi-candidate source', async () => {
    const onApply = jest.fn().mockResolvedValue(undefined)
    const proposal = create(ResyncProposalSchema, {
      bookId: 'b1',
      library: create(SourceBookSchema, { source: '', title: 'Dune' }),
      sources: [
        create(SourceBookSchema, { source: 'hardcover', title: 'Candidate A', index: 0 }),
        create(SourceBookSchema, { source: 'hardcover', title: 'Candidate B', index: 1 })
      ]
    })
    render(<SourceCompare proposal={proposal} onApply={onApply} applyLabel={() => 'Apply'} />)

    fireEvent.click(screen.getByRole('radio', { name: 'Hardcover #2' }))
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() => expect(onApply).toHaveBeenCalledWith('hardcover', 1))
  })
})
