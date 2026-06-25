import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockToggleTag = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useBacklog', () => ({
  useToggleTag: () => mockToggleTag
}))

import BookOwnershipToggles from '@/components/backlog/BookOwnershipToggles'

function makeBook(tags: string[] = [], formats: string[] = []) {
  return create(UserBookSchema, {
    id: 'ub-1',
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

  it('always renders Physical and Digital chips', () => {
    render(<BookOwnershipToggles userBook={makeBook()} />)
    expect(screen.getByText('Physical')).toBeInTheDocument()
    expect(screen.getByText('Digital')).toBeInTheDocument()
  })

  it('toggles own-physical on when clicked from off', async () => {
    render(<BookOwnershipToggles userBook={makeBook()} />)

    fireEvent.click(screen.getByRole('button', { name: /physical/i }))

    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('ub-1', 'own-physical')
    })
    expect(mockMutate).toHaveBeenCalledWith('/backlog/books')
  })

  it('toggles own-digital on when clicked from off', async () => {
    render(<BookOwnershipToggles userBook={makeBook()} />)

    fireEvent.click(screen.getByRole('button', { name: /digital/i }))

    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('ub-1', 'own-digital')
    })
  })

  it('Physical chip is pressed when own-physical tag present', () => {
    render(<BookOwnershipToggles userBook={makeBook(['own-physical'])} />)
    expect(screen.getByRole('button', { name: /physical/i })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
  })

  it('Digital chip is pressed when own-digital tag present', () => {
    render(<BookOwnershipToggles userBook={makeBook(['own-digital'])} />)
    expect(screen.getByRole('button', { name: /digital/i })).toHaveAttribute('aria-pressed', 'true')
  })

  it('shows PDF and EPUB format badges', () => {
    render(<BookOwnershipToggles userBook={makeBook(['own-digital'], ['pdf', 'epub'])} />)
    expect(screen.getByText('PDF')).toBeInTheDocument()
    expect(screen.getByText('EPUB')).toBeInTheDocument()
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

    // Not pressed before
    expect(screen.getByRole('button', { name: /physical/i })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
    fireEvent.click(screen.getByRole('button', { name: /physical/i }))

    await waitFor(() => {
      // Reverted
      expect(screen.getByRole('button', { name: /physical/i })).toHaveAttribute(
        'aria-pressed',
        'false'
      )
    })
  })
})
