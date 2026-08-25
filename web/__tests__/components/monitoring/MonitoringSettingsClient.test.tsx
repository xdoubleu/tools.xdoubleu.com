import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import {
  GetNotificationSettingsResponseSchema,
  ListOAuthConnectionsResponseSchema
} from '@/lib/gen/observability/v1/observability_pb'
import MonitoringSettingsClient from '@/components/monitoring/MonitoringSettingsClient'

const mockUseNotificationSettings = jest.fn()
const mockUseOAuthConnections = jest.fn()
const mockMutate = jest.fn()
const mockRouterReplace = jest.fn()
let mockSearchParams = new URLSearchParams()

jest.mock('next/navigation', () => ({
  useRouter: () => ({ replace: mockRouterReplace }),
  useSearchParams: () => mockSearchParams
}))

jest.mock('@/hooks/useMonitoring', () => ({
  useNotificationSettings: () => mockUseNotificationSettings(),
  useUpdateNotificationSettings: () => jest.fn(),
  useOAuthConnections: () => mockUseOAuthConnections(),
  useDisconnectOAuthConnection: () => jest.fn()
}))

beforeEach(() => {
  jest.clearAllMocks()
  mockSearchParams = new URLSearchParams()
  mockMutate.mockResolvedValue(undefined)
  mockUseNotificationSettings.mockReturnValue({
    data: create(GetNotificationSettingsResponseSchema, {
      settings: [
        { sourceKey: 'sentry_issues', enabled: true },
        { sourceKey: 'failing_dependency_prs', enabled: true }
      ]
    }),
    mutate: mockMutate
  })
  mockUseOAuthConnections.mockReturnValue({
    data: create(ListOAuthConnectionsResponseSchema, { connections: [] }),
    mutate: mockMutate
  })
})

describe('MonitoringSettingsClient', () => {
  it('renders the notification settings and integrations cards', () => {
    render(<MonitoringSettingsClient />)
    expect(screen.getByText('Email notifications')).toBeInTheDocument()
    expect(screen.getByText('Integrations')).toBeInTheDocument()
  })

  it('shows a success banner and revalidates connections on oauth_connected', async () => {
    mockSearchParams = new URLSearchParams('oauth_connected=github')

    render(<MonitoringSettingsClient />)

    expect(await screen.findByText('Connected github.')).toBeInTheDocument()
    expect(mockMutate).toHaveBeenCalled()
    expect(mockRouterReplace).toHaveBeenCalledWith('/monitoring/settings')
  })

  it('shows an error banner on oauth_error without revalidating', async () => {
    mockSearchParams = new URLSearchParams('oauth_error=github')

    render(<MonitoringSettingsClient />)

    expect(
      await screen.findByText('Failed to connect github. Check the server logs for details.')
    ).toBeInTheDocument()
    expect(mockRouterReplace).toHaveBeenCalledWith('/monitoring/settings')
  })

  it('shows no banner and does not touch the URL when neither param is present', () => {
    render(<MonitoringSettingsClient />)

    expect(screen.queryByText(/Connected|Failed to connect/)).not.toBeInTheDocument()
    expect(mockRouterReplace).not.toHaveBeenCalled()
  })
})
