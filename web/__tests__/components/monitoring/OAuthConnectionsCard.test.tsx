import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { ListOAuthConnectionsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import OAuthConnectionsCard from '@/components/monitoring/OAuthConnectionsCard'

const mockDisconnect = vi.fn()

vi.mock('@/hooks/useMonitoring', () => ({
  useDisconnectOAuthConnection: () => mockDisconnect
}))

vi.mock('@/components/monitoring/ProviderConfigDialog', () => ({
  __esModule: true,
  default: ({
    provider,
    open,
    onOpenChange
  }: {
    provider: string
    open: boolean
    onOpenChange: (open: boolean) => void
  }) =>
    open ? (
      <div data-testid="provider-config-dialog">
        Configuring {provider}
        <button onClick={() => onOpenChange(false)}>Close</button>
        <button onClick={() => onOpenChange(true)}>Reopen</button>
      </div>
    ) : null
}))

const mockOnConfiguringProviderChange = vi.fn()

beforeEach(() => {
  vi.clearAllMocks()
})

function renderCard(props: Partial<React.ComponentProps<typeof OAuthConnectionsCard>> = {}) {
  return render(
    <OAuthConnectionsCard
      data={undefined}
      configuringProvider={null}
      onConfiguringProviderChange={mockOnConfiguringProviderChange}
      {...props}
    />
  )
}

describe('OAuthConnectionsCard', () => {
  it('shows a loading state without data', () => {
    renderCard()
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('renders a connected provider with its connector and a disconnect action', async () => {
    const data = create(ListOAuthConnectionsResponseSchema, {
      connections: [
        {
          provider: 'github',
          connected: true,
          connectedBy: 'admin@example.com',
          connectedAt: '2026-01-01T00:00:00Z',
          expiresAt: ''
        }
      ]
    })

    renderCard({ data })
    expect(screen.getByText('GitHub')).toBeInTheDocument()
    expect(screen.getByText('Connected')).toBeInTheDocument()
    expect(screen.getByText(/admin@example.com/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Disconnect' }))
    expect(mockDisconnect).not.toHaveBeenCalled()
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveTextContent('Disconnect GitHub?')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Disconnect' }))
    expect(mockDisconnect).toHaveBeenCalledWith('github')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('reports a failed disconnect', async () => {
    mockDisconnect.mockRejectedValueOnce(new Error('boom'))
    const data = create(ListOAuthConnectionsResponseSchema, {
      connections: [{ provider: 'sentry', connected: true, connectedBy: 'a', connectedAt: '' }]
    })
    renderCard({ data })
    fireEvent.click(screen.getByRole('button', { name: 'Disconnect' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Disconnect' }))
    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't disconnect Sentry.")
  })

  it('closes the confirm on cancel without disconnecting', async () => {
    const data = create(ListOAuthConnectionsResponseSchema, {
      connections: [{ provider: 'other', connected: true, connectedBy: 'a', connectedAt: '' }]
    })
    renderCard({ data })
    fireEvent.click(screen.getByRole('button', { name: 'Disconnect' }))
    expect(screen.getByRole('dialog')).toHaveTextContent('Disconnect other?')
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(mockDisconnect).not.toHaveBeenCalled()

    mockDisconnect.mockRejectedValueOnce(new Error('boom'))
    fireEvent.click(screen.getByRole('button', { name: 'Disconnect' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Disconnect' }))
    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't disconnect other.")
  })

  it('opens the config dialog for a connected provider', () => {
    const data = create(ListOAuthConnectionsResponseSchema, {
      connections: [
        {
          provider: 'github',
          connected: true,
          connectedBy: 'admin@example.com',
          connectedAt: '2026-01-01T00:00:00Z',
          expiresAt: ''
        }
      ]
    })

    renderCard({ data })
    expect(screen.queryByTestId('provider-config-dialog')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Configure' }))
    expect(mockOnConfiguringProviderChange).toHaveBeenCalledWith('github')
  })

  it('shows the config dialog when configuringProvider is set from a parent', () => {
    const data = create(ListOAuthConnectionsResponseSchema, {
      connections: [
        {
          provider: 'sentry',
          connected: true,
          connectedBy: 'admin@example.com',
          connectedAt: '2026-01-01T00:00:00Z',
          expiresAt: ''
        }
      ]
    })

    renderCard({ data, configuringProvider: 'sentry' })
    expect(screen.getByTestId('provider-config-dialog')).toHaveTextContent('Configuring sentry')

    fireEvent.click(screen.getByRole('button', { name: 'Reopen' }))
    expect(mockOnConfiguringProviderChange).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(mockOnConfiguringProviderChange).toHaveBeenCalledWith(null)
  })

  it('renders an unconnected provider with a connect link', () => {
    const data = create(ListOAuthConnectionsResponseSchema, {
      connections: [
        {
          provider: 'sentry',
          connected: false,
          connectedBy: '',
          connectedAt: '',
          expiresAt: ''
        }
      ]
    })

    renderCard({ data })
    expect(screen.getByText('Sentry')).toBeInTheDocument()
    expect(screen.getByText('Not connected')).toBeInTheDocument()

    const link = screen.getByRole('link', { name: 'Connect' })
    expect(link).toHaveAttribute('href', expect.stringContaining('/admin/oauth/sentry/start'))
  })
})
