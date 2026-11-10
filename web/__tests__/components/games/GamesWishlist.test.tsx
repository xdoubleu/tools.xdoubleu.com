import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

vi.mock('@/hooks/useGames', () => ({ useWishlist: vi.fn() }))

import GamesWishlist from '@/components/games/GamesWishlist'
import { useWishlist } from '@/hooks/useGames'
import { GetWishlistResponseSchema, WishlistGameSchema } from '@/lib/gen/games/v1/games_pb'

const mockUseWishlist = vi.mocked(useWishlist)

function mockState(state: { data?: unknown; error?: Error }) {
  // @ts-expect-error -- partial SWRResponse
  mockUseWishlist.mockReturnValue(state)
}

describe('GamesWishlist', () => {
  it('lists wishlisted games linking to the store', () => {
    mockState({
      data: create(GetWishlistResponseSchema, {
        games: [
          create(WishlistGameSchema, { id: 620, name: 'Portal 2', imageUrl: 'https://i/p.jpg' })
        ]
      })
    })
    render(<GamesWishlist />)
    expect(screen.getByText('Wishlist (1)')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Portal 2' })).toHaveAttribute(
      'href',
      'https://store.steampowered.com/app/620'
    )
  })

  it('renders nothing while empty or loading', () => {
    mockState({})
    const { container } = render(<GamesWishlist />)
    expect(container).toBeEmptyDOMElement()
  })

  it('shows an error state when the fetch fails', () => {
    mockState({ error: new Error('boom') })
    render(<GamesWishlist />)
    expect(screen.getByText(/Steam wishlist/i)).toBeInTheDocument()
  })
})
