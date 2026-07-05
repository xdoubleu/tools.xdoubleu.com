import { render, screen } from '@testing-library/react'

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/games/GamesSettingsClient', () => () => (
  <div data-testid="games-settings-client" />
))

import BacklogGamesSettingsPage from '@/app/games/settings/page'

describe('BacklogGamesSettingsPage', () => {
  it('server-fetches integrations and renders the client component', async () => {
    render(await BacklogGamesSettingsPage())
    expect(screen.getByTestId('games-settings-client')).toBeInTheDocument()
  })
})
