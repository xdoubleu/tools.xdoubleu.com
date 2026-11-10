import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

import GamesWishlist from '@/components/games/GamesWishlist'
import { WishlistGameSchema } from '@/lib/gen/games/v1/games_pb'

describe('GamesWishlist', () => {
  it('lists wishlisted games linking to the store', () => {
    const { container } = render(
      <GamesWishlist
        games={[
          create(WishlistGameSchema, { id: 620, name: 'Portal 2', imageUrl: 'https://i/p.jpg' }),
          create(WishlistGameSchema, { id: 10, name: 'No Art' })
        ]}
      />
    )
    const images = container.querySelectorAll('img')
    expect(images).toHaveLength(1)
    expect(images[0]).toHaveAttribute('src', 'https://i/p.jpg')
    expect(screen.getByText('Wishlist (2)')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Portal 2' })).toHaveAttribute(
      'href',
      'https://store.steampowered.com/app/620'
    )
  })

  it('renders nothing while empty', () => {
    const { container } = render(<GamesWishlist games={[]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('says so when empty and asked to show it', () => {
    render(<GamesWishlist games={[]} showEmpty />)
    expect(screen.getByText('Nothing on your wishlist.')).toBeInTheDocument()
  })
})
