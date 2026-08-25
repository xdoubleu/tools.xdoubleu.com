import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { GetNotificationSettingsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import NotificationSettingsCard from '@/components/monitoring/NotificationSettingsCard'

jest.mock('@/hooks/useMonitoring', () => ({
  useUpdateNotificationSettings: () => jest.fn()
}))

describe('NotificationSettingsCard', () => {
  it('renders only the monitoring-owned sources, not unhealthy_feeds', () => {
    const data = create(GetNotificationSettingsResponseSchema, {
      settings: [
        { sourceKey: 'sentry_issues', enabled: true },
        { sourceKey: 'failing_dependency_prs', enabled: false },
        { sourceKey: 'unhealthy_feeds', enabled: true }
      ]
    })
    render(<NotificationSettingsCard data={data} />)

    expect(screen.getByText('Sentry issues')).toBeInTheDocument()
    expect(screen.getByText('Failing dependency PRs')).toBeInTheDocument()
    expect(screen.queryByText('Unhealthy feeds')).not.toBeInTheDocument()
  })

  it('shows a loading state without data', () => {
    render(<NotificationSettingsCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })
})
