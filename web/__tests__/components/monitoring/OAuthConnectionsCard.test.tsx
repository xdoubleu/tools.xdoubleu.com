import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent } from '@testing-library/react'
import { ListOAuthConnectionsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import OAuthConnectionsCard from '@/components/monitoring/OAuthConnectionsCard'

const mockDisconnect = jest.fn()

jest.mock('@/hooks/useMonitoring', () => ({
  useDisconnectOAuthConnection: () => mockDisconnect
}))

jest.mock('@/components/monitoring/ProviderConfigDialog', () => ({
  __esModule: true,
  default: ({ provider, open }: { provider: string; open: boolean }) =>
    open ? <div data-testid="provider-config-dialog">Configuring {provider}</div> : null
}))

beforeEach(() => {
  jest.clearAllMocks()
})

describe('OAuthConnectionsCard', () => {
  it('shows a loading state without data', () => {
    render(<OAuthConnectionsCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('renders a connected provider with its connector and a disconnect action', () => {
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

    render(<OAuthConnectionsCard data={data} />)
    expect(screen.getByText('GitHub')).toBeInTheDocument()
    expect(screen.getByText('Connected')).toBeInTheDocument()
    expect(screen.getByText(/admin@example.com/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Disconnect' }))
    expect(mockDisconnect).toHaveBeenCalledWith('github')
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

    render(<OAuthConnectionsCard data={data} />)
    expect(screen.queryByTestId('provider-config-dialog')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Configure' }))
    expect(screen.getByTestId('provider-config-dialog')).toHaveTextContent('Configuring github')
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

    render(<OAuthConnectionsCard data={data} />)
    expect(screen.getByText('Sentry')).toBeInTheDocument()
    expect(screen.getByText('Not connected')).toBeInTheDocument()

    const link = screen.getByRole('link', { name: 'Connect' })
    expect(link).toHaveAttribute('href', expect.stringContaining('/admin/oauth/sentry/start'))
  })
})
