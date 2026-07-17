import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/reading/v1/library_pb'

const mockToggleTag = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useBooks', () => ({
  useToggleTag: () => mockToggleTag
}))

import BookOwnershipToggles from '@/components/reading/BookOwnershipToggles'

function makeBook(tags: string[] = [], formats: string[] = []) {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status: 'to-read',
    tags,
    formats,
    book: create(BookSchema, { title: 'T', authors: [] })
  })
}

describe('BookOwnershipToggles', () => {
  beforeEach(() => {
    mockToggleTag.mockReset()
    mockMutate.mockReset()
    mockToggleTag.mockResolvedValue({})
  })

  it('always renders Physical chip', () => {
    render(<BookOwnershipToggles userBook={makeBook()} />)
    expect(screen.getByText('Physical')).toBeInTheDocument()
  })

  it('renders the Ownership label by default', () => {
    render(<BookOwnershipToggles userBook={makeBook()} />)
    expect(screen.getByText('Ownership')).toBeInTheDocument()
  })

  it('hides the Ownership label when hideLabel is set', () => {
    render(<BookOwnershipToggles userBook={makeBook()} hideLabel />)
    expect(screen.queryByText('Ownership')).not.toBeInTheDocument()
  })

  it('does not render a Digital toggle', () => {
    render(<BookOwnershipToggles userBook={makeBook()} />)
    expect(screen.queryByRole('button', { name: /digital/i })).not.toBeInTheDocument()
  })

  it('toggles own-physical on when clicked from off', async () => {
    render(<BookOwnershipToggles userBook={makeBook()} />)

    fireEvent.click(screen.getByRole('button', { name: /physical/i }))

    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('book-1', 'own-physical')
    })
    expect(mockMutate).toHaveBeenCalledWith('/reading')
  })

  it('Physical chip is pressed when own-physical tag present', () => {
    render(<BookOwnershipToggles userBook={makeBook(['own-physical'])} />)
    expect(screen.getByRole('button', { name: /physical/i })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
  })

  it('Physical chip is not pressed when own-physical tag absent', () => {
    render(<BookOwnershipToggles userBook={makeBook()} />)
    expect(screen.getByRole('button', { name: /physical/i })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
  })

  it('shows PDF badge when pdf format present', () => {
    render(<BookOwnershipToggles userBook={makeBook([], ['pdf'])} />)
    expect(screen.getByText('PDF')).toBeInTheDocument()
  })

  it('shows EPUB badge when epub format present', () => {
    render(<BookOwnershipToggles userBook={makeBook([], ['epub'])} />)
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('shows both PDF and EPUB badges when both formats present', () => {
    render(<BookOwnershipToggles userBook={makeBook([], ['pdf', 'epub'])} />)
    expect(screen.getByText('PDF')).toBeInTheDocument()
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('shows no format badges when formats is empty', () => {
    render(<BookOwnershipToggles userBook={makeBook()} />)
    expect(screen.queryByText('PDF')).not.toBeInTheDocument()
    expect(screen.queryByText('EPUB')).not.toBeInTheDocument()
  })

  it('calls onSaved after toggle', async () => {
    const onSaved = jest.fn()
    render(<BookOwnershipToggles userBook={makeBook()} onSaved={onSaved} />)

    fireEvent.click(screen.getByRole('button', { name: /physical/i }))

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalled()
    })
  })

  it('reverts optimistic state on error', async () => {
    mockToggleTag.mockRejectedValue(new Error('fail'))
    render(<BookOwnershipToggles userBook={makeBook()} />)

    expect(screen.getByRole('button', { name: /physical/i })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
    fireEvent.click(screen.getByRole('button', { name: /physical/i }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /physical/i })).toHaveAttribute(
        'aria-pressed',
        'false'
      )
    })
  })
})
