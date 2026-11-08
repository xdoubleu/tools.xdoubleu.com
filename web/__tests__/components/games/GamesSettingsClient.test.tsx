import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const mockSaveSettings = vi.fn()

const loaded = {
  data: { integrations: { steamUserId: '12345678' } },
  isLoading: false,
  error: null as Error | null
}
const mockUseIntegrations = vi.fn(() => loaded)

vi.mock('@/hooks/useGames', () => ({
  useIntegrations: () => mockUseIntegrations(),
  useSaveIntegrations: () => mockSaveSettings
}))

vi.mock('swr', () => ({ __esModule: true, mutate: vi.fn(), default: vi.fn() }))

vi.mock('@/lib/gen/games/v1/games_pb', () => ({
  Integrations: {}
}))

import GamesSettingsClient from '@/components/games/GamesSettingsClient'

describe('GamesSettingsClient', () => {
  beforeEach(() => {
    mockSaveSettings.mockReset()
    mockUseIntegrations.mockReturnValue(loaded)
  })

  it('shows loading and error states', () => {
    mockUseIntegrations.mockReturnValue({ ...loaded, isLoading: true })
    const { rerender } = render(<GamesSettingsClient />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()

    mockUseIntegrations.mockReturnValue({ ...loaded, error: new Error('x') })
    rerender(<GamesSettingsClient />)
    expect(screen.getByText('Failed to load settings.')).toBeInTheDocument()
  })

  it('renders the Games settings heading', () => {
    render(<GamesSettingsClient />)
    expect(screen.getByRole('heading', { name: 'Games settings' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /dashboard/games', () => {
    render(<GamesSettingsClient />)
    expect(screen.getByRole('link', { name: 'Games' })).toHaveAttribute('href', '/dashboard/games')
  })

  it('renders the Steam section', () => {
    render(<GamesSettingsClient />)
    expect(screen.getByText('Steam')).toBeInTheDocument()
  })

  it('renders the Steam user ID input and takes edits', () => {
    render(<GamesSettingsClient />)
    const input = screen.getByLabelText('Steam user ID')
    fireEvent.change(input, { target: { value: '999' } })
    expect(input).toHaveValue('999')
  })

  it('renders the Save button', () => {
    render(<GamesSettingsClient />)
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument()
  })

  it('shows success message after saving', async () => {
    mockSaveSettings.mockResolvedValueOnce(undefined)
    render(<GamesSettingsClient />)
    fireEvent.submit(screen.getByRole('button', { name: 'Save' }).closest('form')!)
    await waitFor(() =>
      expect(screen.getByText('Settings saved successfully.')).toBeInTheDocument()
    )
  })

  it('shows error message when save fails', async () => {
    mockSaveSettings.mockRejectedValueOnce(new Error('network error'))
    render(<GamesSettingsClient />)
    fireEvent.submit(screen.getByRole('button', { name: 'Save' }).closest('form')!)
    await waitFor(() => expect(screen.getByText('Failed to save settings.')).toBeInTheDocument())
  })

  it('does not render Kobo or import sections', () => {
    render(<GamesSettingsClient />)
    expect(screen.queryByText('Kobo')).not.toBeInTheDocument()
    expect(screen.queryByText('Import books')).not.toBeInTheDocument()
    expect(screen.queryByText('Danger zone')).not.toBeInTheDocument()
  })
})
