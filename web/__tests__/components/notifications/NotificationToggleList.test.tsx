import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { GetNotificationSettingsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import NotificationToggleList from '@/components/notifications/NotificationToggleList'

const mockUpdateNotificationSettings = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useUpdateNotificationSettings: () => mockUpdateNotificationSettings
}))

beforeEach(() => {
  mockUpdateNotificationSettings.mockReset()
  mockUpdateNotificationSettings.mockResolvedValue(undefined)
})

describe('NotificationToggleList', () => {
  it('shows a loading state without data', () => {
    render(<NotificationToggleList data={undefined} sourceKeys={['sentry_issues']} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('renders a labeled row with a checkbox per allowlisted source', () => {
    const data = create(GetNotificationSettingsResponseSchema, {
      settings: [
        { sourceKey: 'sentry_issues', enabled: true },
        { sourceKey: 'failing_dependency_prs', enabled: false },
        { sourceKey: 'unhealthy_feeds', enabled: true }
      ]
    })
    render(
      <NotificationToggleList
        data={data}
        sourceKeys={['sentry_issues', 'failing_dependency_prs']}
      />
    )

    expect(screen.getByText('Sentry issues')).toBeInTheDocument()
    expect(screen.getByText('Failing dependency PRs')).toBeInTheDocument()
    expect(screen.queryByText('Unhealthy feeds')).not.toBeInTheDocument()
    expect(
      screen.getByRole('checkbox', { name: 'Email notifications for Sentry issues' })
    ).toBeChecked()
    expect(
      screen.getByRole('checkbox', { name: 'Email notifications for Failing dependency PRs' })
    ).not.toBeChecked()
  })

  it('falls back to the raw source key for an unrecognized source', () => {
    const data = create(GetNotificationSettingsResponseSchema, {
      settings: [{ sourceKey: 'some_new_source', enabled: true }]
    })
    render(<NotificationToggleList data={data} sourceKeys={['some_new_source']} />)
    expect(screen.getByText('some_new_source')).toBeInTheDocument()
  })

  it('toggles a source and disables it while pending', async () => {
    const data = create(GetNotificationSettingsResponseSchema, {
      settings: [{ sourceKey: 'sentry_issues', enabled: true }]
    })
    render(<NotificationToggleList data={data} sourceKeys={['sentry_issues']} />)

    const checkbox = screen.getByRole('checkbox', {
      name: 'Email notifications for Sentry issues'
    })
    fireEvent.click(checkbox)

    expect(mockUpdateNotificationSettings).toHaveBeenCalledWith('sentry_issues', false)
    await waitFor(() => expect(checkbox).not.toBeDisabled())
  })
})
