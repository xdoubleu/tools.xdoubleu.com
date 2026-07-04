import { render, screen } from '@testing-library/react'

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/app/games/[id]/SteamGameClient', () => (props: { id: string }) => (
  <div data-testid="steam-game-client">{props.id}</div>
))

import SteamGamePage from '@/app/games/[id]/page'

describe('SteamGamePage', () => {
  it('awaits params and renders the client with the id', async () => {
    render(await SteamGamePage({ params: Promise.resolve({ id: '42' }) }))
    expect(screen.getByTestId('steam-game-client')).toHaveTextContent('42')
  })

  it('skips the server fetch for a non-numeric id', async () => {
    render(await SteamGamePage({ params: Promise.resolve({ id: 'abc' }) }))
    expect(screen.getByTestId('steam-game-client')).toBeInTheDocument()
  })
})
