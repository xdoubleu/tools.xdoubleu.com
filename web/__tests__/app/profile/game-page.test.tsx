import React from 'react'
import { render, screen } from '@testing-library/react'

const mockProfileGameClient = jest.fn<React.JSX.Element, [object]>(() => (
  <div data-testid="profile-game" />
))
jest.mock('@/components/profile/ProfileGameClient', () => ({
  __esModule: true,
  default: (props: object) => mockProfileGameClient(props)
}))

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

import ProfileGamePage, { metadata } from '@/app/profile/games/[token]/[id]/page'

describe('ProfileGamePage', () => {
  it('renders the client component with token and id', async () => {
    render(await ProfileGamePage({ params: Promise.resolve({ token: 'tok-1', id: '7' }) }))
    expect(screen.getByTestId('profile-game')).toBeInTheDocument()
    expect(mockProfileGameClient).toHaveBeenCalledWith(
      expect.objectContaining({ token: 'tok-1', id: '7' })
    )
  })

  it('is excluded from search indexing', () => {
    expect(metadata.robots).toEqual({ index: false, follow: false })
  })
})
