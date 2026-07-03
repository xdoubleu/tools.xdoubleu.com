import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const mockSaveSettings = jest.fn()

jest.mock('@/hooks/useGames', () => ({
  useIntegrations: () => ({
    data: { integrations: { steamUserId: '12345678' } },
    isLoading: false,
    error: null
  }),
  useSaveIntegrations: () => mockSaveSettings
}))

jest.mock('swr', () => ({ __esModule: true, mutate: jest.fn(), default: jest.fn() }))

jest.mock('@/lib/gen/games/v1/games_pb', () => ({
  Integrations: {}
}))

import BacklogGamesSettingsPage from '@/app/games/settings/page'

describe('BacklogGamesSettingsPage', () => {
  beforeEach(() => {
    mockSaveSettings.mockReset()
  })

  it('renders the Games Settings heading', () => {
    render(<BacklogGamesSettingsPage />)
    expect(screen.getByRole('heading', { name: 'Games Settings' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /games', () => {
    render(<BacklogGamesSettingsPage />)
    expect(screen.getByRole('link', { name: 'Games' })).toHaveAttribute('href', '/games')
  })

  it('renders the Steam section', () => {
    render(<BacklogGamesSettingsPage />)
    expect(screen.getByText('Steam')).toBeInTheDocument()
  })

  it('renders the Steam User ID input', () => {
    render(<BacklogGamesSettingsPage />)
    expect(screen.getByLabelText('Steam User ID')).toBeInTheDocument()
  })

  it('renders the Save button', () => {
    render(<BacklogGamesSettingsPage />)
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument()
  })

  it('shows success message after saving', async () => {
    mockSaveSettings.mockResolvedValueOnce(undefined)
    render(<BacklogGamesSettingsPage />)
    fireEvent.submit(screen.getByRole('button', { name: 'Save' }).closest('form')!)
    await waitFor(() =>
      expect(screen.getByText('Settings saved successfully.')).toBeInTheDocument()
    )
  })

  it('shows error message when save fails', async () => {
    mockSaveSettings.mockRejectedValueOnce(new Error('network error'))
    render(<BacklogGamesSettingsPage />)
    fireEvent.submit(screen.getByRole('button', { name: 'Save' }).closest('form')!)
    await waitFor(() => expect(screen.getByText('Failed to save settings.')).toBeInTheDocument())
  })

  it('does not render Kobo or import sections', () => {
    render(<BacklogGamesSettingsPage />)
    expect(screen.queryByText('Kobo')).not.toBeInTheDocument()
    expect(screen.queryByText('Import books')).not.toBeInTheDocument()
    expect(screen.queryByText('Danger zone')).not.toBeInTheDocument()
  })
})
