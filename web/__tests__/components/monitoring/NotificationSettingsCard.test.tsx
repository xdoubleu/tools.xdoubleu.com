import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { GetNotificationSettingsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import NotificationSettingsCard from '@/components/monitoring/NotificationSettingsCard'

const mockUpdateNotificationSettings = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useUpdateNotificationSettings: () => mockUpdateNotificationSettings
}))

beforeEach(() => {
  mockUpdateNotificationSettings.mockReset()
  mockUpdateNotificationSettings.mockResolvedValue(undefined)
})

describe('NotificationSettingsCard', () => {
  it('shows a loading state without data', () => {
    render(<NotificationSettingsCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('renders a labeled row with a checkbox per source', () => {
    const data = create(GetNotificationSettingsResponseSchema, {
      settings: [
        { sourceKey: 'sentry_issues', enabled: true },
        { sourceKey: 'failing_dependency_prs', enabled: false }
      ]
    })
    render(<NotificationSettingsCard data={data} />)

    expect(screen.getByText('Sentry issues')).toBeInTheDocument()
    expect(screen.getByText('Failing dependency PRs')).toBeInTheDocument()
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
    render(<NotificationSettingsCard data={data} />)
    expect(screen.getByText('some_new_source')).toBeInTheDocument()
  })

  it('toggles a source and disables it while pending', async () => {
    const data = create(GetNotificationSettingsResponseSchema, {
      settings: [{ sourceKey: 'sentry_issues', enabled: true }]
    })
    render(<NotificationSettingsCard data={data} />)

    const checkbox = screen.getByRole('checkbox', {
      name: 'Email notifications for Sentry issues'
    })
    fireEvent.click(checkbox)

    expect(mockUpdateNotificationSettings).toHaveBeenCalledWith('sentry_issues', false)
    await waitFor(() => expect(checkbox).not.toBeDisabled())
  })
})
