import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { GetNotificationSettingsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import FeedsNotificationSettingsCard from '@/components/feeds/FeedsNotificationSettingsCard'

const mockUseNotificationSettings = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useNotificationSettings: () => mockUseNotificationSettings(),
  useUpdateNotificationSettings: () => jest.fn()
}))

describe('FeedsNotificationSettingsCard', () => {
  it('renders only unhealthy_feeds, not the monitoring-owned sources', () => {
    mockUseNotificationSettings.mockReturnValue({
      data: create(GetNotificationSettingsResponseSchema, {
        settings: [
          { sourceKey: 'sentry_issues', enabled: true },
          { sourceKey: 'failing_dependency_prs', enabled: true },
          { sourceKey: 'unhealthy_feeds', enabled: false }
        ]
      })
    })
    render(<FeedsNotificationSettingsCard />)

    expect(screen.getByText('Unhealthy feeds')).toBeInTheDocument()
    expect(screen.queryByText('Sentry issues')).not.toBeInTheDocument()
    expect(screen.queryByText('Failing dependency PRs')).not.toBeInTheDocument()
    expect(
      screen.getByRole('checkbox', { name: 'Email notifications for Unhealthy feeds' })
    ).not.toBeChecked()
  })

  it('shows a loading state without data', () => {
    mockUseNotificationSettings.mockReturnValue({ data: undefined })
    render(<FeedsNotificationSettingsCard />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })
})
