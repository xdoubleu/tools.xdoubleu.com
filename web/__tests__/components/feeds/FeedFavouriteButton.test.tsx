import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const updateItem = jest.fn()
jest.mock('@/hooks/useFeeds', () => ({
  useUpdateItem: () => updateItem
}))

import FeedFavouriteButton from '@/components/feeds/FeedFavouriteButton'

describe('FeedFavouriteButton', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('marks an item as favourite optimistically and persists it', async () => {
    updateItem.mockResolvedValue({})
    render(<FeedFavouriteButton itemId="item-1" favourite={false} />)

    const button = screen.getByRole('button', { name: 'Add to favourites' })
    fireEvent.click(button)

    expect(screen.getByRole('button', { name: 'Remove from favourites' })).toBeInTheDocument()
    await waitFor(() => {
      expect(updateItem).toHaveBeenCalledWith('item-1', { favourite: true })
    })
  })

  it('reverts the optimistic update when the mutation fails', async () => {
    updateItem.mockRejectedValue(new Error('nope'))
    render(<FeedFavouriteButton itemId="item-1" favourite={false} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add to favourites' }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Add to favourites' })).toBeInTheDocument()
    })
  })

  it('unmarks an already-favourite item', async () => {
    updateItem.mockResolvedValue({})
    render(<FeedFavouriteButton itemId="item-1" favourite={true} />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove from favourites' }))

    await waitFor(() => {
      expect(updateItem).toHaveBeenCalledWith('item-1', { favourite: false })
    })
  })
})
