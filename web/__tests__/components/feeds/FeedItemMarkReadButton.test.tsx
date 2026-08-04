import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const mockUpdateItem = jest.fn()

jest.mock('@/hooks/useFeeds', () => ({
  useUpdateItem: () => mockUpdateItem
}))

import FeedItemMarkReadButton from '@/components/feeds/FeedItemMarkReadButton'

describe('FeedItemMarkReadButton', () => {
  beforeEach(() => {
    jest.useFakeTimers()
    mockUpdateItem.mockReset()
    mockUpdateItem.mockResolvedValue({})
  })

  afterEach(() => {
    jest.useRealTimers()
  })

  it('renders a "Mark read" button', () => {
    render(<FeedItemMarkReadButton itemId="item-1" onMarkRead={jest.fn()} onSettled={jest.fn()} />)
    expect(screen.getByRole('button', { name: 'Mark read' })).toBeInTheDocument()
  })

  it('marks the item read and shows an Undo affordance', async () => {
    render(<FeedItemMarkReadButton itemId="item-1" onMarkRead={jest.fn()} onSettled={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))

    await waitFor(() => {
      expect(mockUpdateItem).toHaveBeenCalledWith('item-1', { read: true })
    })
    expect(screen.getByRole('button', { name: 'Undo' })).toBeInTheDocument()
  })

  it('calls onMarkRead synchronously on click, before the mutation resolves', () => {
    const onMarkRead = jest.fn()
    render(<FeedItemMarkReadButton itemId="item-1" onMarkRead={onMarkRead} onSettled={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))

    expect(onMarkRead).toHaveBeenCalledWith('item-1')
  })

  it('calls onSettled once the undo window elapses', async () => {
    const onSettled = jest.fn()
    render(<FeedItemMarkReadButton itemId="item-1" onMarkRead={jest.fn()} onSettled={onSettled} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))
    await waitFor(() => screen.getByRole('button', { name: 'Undo' }))

    jest.advanceTimersByTime(4000)

    expect(onSettled).toHaveBeenCalledWith('item-1')
  })

  it('reverts to the prior state and never settles when Undo is clicked', async () => {
    const onSettled = jest.fn()
    render(<FeedItemMarkReadButton itemId="item-1" onMarkRead={jest.fn()} onSettled={onSettled} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))
    await waitFor(() => screen.getByRole('button', { name: 'Undo' }))

    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))

    await waitFor(() => {
      expect(mockUpdateItem).toHaveBeenLastCalledWith('item-1', { read: false })
    })
    expect(screen.getByRole('button', { name: 'Mark read' })).toBeInTheDocument()

    jest.advanceTimersByTime(4000)
    expect(onSettled).not.toHaveBeenCalled()
  })

  it('reverts to the button on error', async () => {
    mockUpdateItem.mockRejectedValueOnce(new Error('fail'))
    render(<FeedItemMarkReadButton itemId="item-1" onMarkRead={jest.fn()} onSettled={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Mark read' })).toBeInTheDocument()
    })
  })
})
