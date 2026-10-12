import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockActions = {
  setFeedback: jest.fn().mockResolvedValue(undefined),
  setSaved: jest.fn().mockResolvedValue(undefined),
  accept: jest.fn().mockResolvedValue(undefined)
}
jest.mock('@/hooks/useTaste', () => ({ usePickActions: () => mockActions }))

import PickActions from '@/components/taste/PickActions'
import { RecommendationSchema } from '@/lib/gen/taste/v1/taste_pb'

const book = create(RecommendationSchema, {
  id: 'r1',
  domain: 'books',
  title: 'Hyperion',
  canAccept: true
})

beforeEach(() => jest.clearAllMocks())

describe('PickActions', () => {
  it('records feedback', async () => {
    render(<PickActions pick={book} />)
    const select = screen.getByLabelText('How did Hyperion land?')
    expect(select).toHaveValue('')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(select).toHaveClass('min-w-0', 'flex-1', 'sm:w-40')
    fireEvent.change(select, { target: { value: 'known' } })
    await waitFor(() => expect(mockActions.setFeedback).toHaveBeenCalledWith('r1', 'known'))
    expect(screen.getByRole('option', { name: 'Already know it' })).toBeInTheDocument()
  })

  it('shows the current feedback', () => {
    render(<PickActions pick={create(RecommendationSchema, { ...book, feedback: 'miss' })} />)
    expect(screen.getByLabelText('How did Hyperion land?')).toHaveValue('miss')
  })

  it('saves and unsaves', async () => {
    const { rerender } = render(<PickActions pick={book} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save Hyperion' }))
    await waitFor(() => expect(mockActions.setSaved).toHaveBeenCalledWith('r1', true))

    rerender(<PickActions pick={create(RecommendationSchema, { ...book, saved: true })} />)
    const unsave = screen.getByRole('button', { name: 'Unsave Hyperion' })
    expect(unsave).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(unsave)
    await waitFor(() => expect(mockActions.setSaved).toHaveBeenCalledWith('r1', false))
  })

  it('adds an acceptable pick to the backlog', async () => {
    const { rerender } = render(<PickActions pick={book} />)
    const add = screen.getByRole('button', { name: 'Add to backlog' })
    expect(add).toHaveClass('min-h-11')
    fireEvent.click(add)
    expect(add).toBeDisabled()
    expect(add).toHaveTextContent('Adding…')
    await waitFor(() => expect(mockActions.accept).toHaveBeenCalledWith('r1'))
    await waitFor(() => expect(add).not.toBeDisabled())
    expect(add).toHaveTextContent('Add to backlog')

    rerender(<PickActions pick={create(RecommendationSchema, { ...book, accepted: true })} />)
    expect(screen.getByText('In your backlog')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Add to backlog' })).not.toBeInTheDocument()

    rerender(<PickActions pick={create(RecommendationSchema, { ...book, canAccept: false })} />)
    expect(screen.queryByRole('button', { name: 'Add to backlog' })).not.toBeInTheDocument()
    expect(screen.queryByText('In your backlog')).not.toBeInTheDocument()
  })

  it('runs one action at a time', async () => {
    render(<PickActions pick={book} />)
    const save = screen.getByRole('button', { name: 'Save Hyperion' })
    fireEvent.click(save)
    expect(screen.getByLabelText('How did Hyperion land?')).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Add to backlog' })).toBeDisabled()
    fireEvent.click(save)
    await waitFor(() => expect(screen.getByLabelText('How did Hyperion land?')).toBeEnabled())
    expect(mockActions.setSaved).toHaveBeenCalledTimes(1)
  })

  it('says when a write failed, and clears it on the next try', async () => {
    mockActions.accept.mockRejectedValueOnce(new Error('nope'))
    render(<PickActions pick={book} />)
    fireEvent.click(screen.getByRole('button', { name: 'Add to backlog' }))
    expect(await screen.findByRole('alert')).toHaveTextContent("That didn't save. Try again.")

    fireEvent.click(screen.getByRole('button', { name: 'Save Hyperion' }))
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })
})
