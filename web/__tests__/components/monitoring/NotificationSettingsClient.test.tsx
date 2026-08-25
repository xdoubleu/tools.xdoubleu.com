import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { GetNotificationSettingsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import NotificationSettingsClient from '@/components/monitoring/NotificationSettingsClient'

const mockUseNotificationSettings = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useNotificationSettings: () => mockUseNotificationSettings(),
  useUpdateNotificationSettings: () => jest.fn()
}))

describe('NotificationSettingsClient', () => {
  it('renders the monitoring-owned sources from the hook data', () => {
    mockUseNotificationSettings.mockReturnValue({
      data: create(GetNotificationSettingsResponseSchema, {
        settings: [
          { sourceKey: 'sentry_issues', enabled: true },
          { sourceKey: 'unhealthy_feeds', enabled: true }
        ]
      })
    })
    render(<NotificationSettingsClient />)

    expect(screen.getByText('Sentry issues')).toBeInTheDocument()
    expect(screen.queryByText('Unhealthy feeds')).not.toBeInTheDocument()
  })
})
