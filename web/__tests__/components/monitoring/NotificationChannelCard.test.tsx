import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { GetNotificationSettingsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import NotificationChannelCard from '@/components/monitoring/NotificationChannelCard'

const updateChannel = jest.fn()

jest.mock('@/hooks/useMonitoring', () => ({
  useUpdateNotificationChannel: () => updateChannel
}))

beforeEach(() => {
  updateChannel.mockReset()
  updateChannel.mockResolvedValue(undefined)
})

describe('NotificationChannelCard', () => {
  it('shows a loading state without data', () => {
    render(<NotificationChannelCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('hides the webhook field while email-only is selected', () => {
    const data = create(GetNotificationSettingsResponseSchema, { channelMode: 'email' })
    render(<NotificationChannelCard data={data} />)
    expect(screen.queryByLabelText(/Slack Incoming Webhook URL/)).not.toBeInTheDocument()
  })

  it('reveals the webhook field and reports it is configured', () => {
    const data = create(GetNotificationSettingsResponseSchema, {
      channelMode: 'both',
      slackWebhookConfigured: true
    })
    render(<NotificationChannelCard data={data} />)
    expect(screen.getByLabelText(/Slack Incoming Webhook URL/)).toBeInTheDocument()
    expect(screen.getByText('A webhook URL is configured.')).toBeInTheDocument()
  })

  it('saves the mode without a URL when the field is untouched', async () => {
    const data = create(GetNotificationSettingsResponseSchema, {
      channelMode: 'email',
      slackWebhookConfigured: true
    })
    render(<NotificationChannelCard data={data} />)

    fireEvent.click(screen.getByLabelText('Slack only'))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(updateChannel).toHaveBeenCalledWith('slack', undefined))
  })

  it('sends the typed webhook URL on save', async () => {
    const data = create(GetNotificationSettingsResponseSchema, { channelMode: 'both' })
    render(<NotificationChannelCard data={data} />)

    fireEvent.change(screen.getByLabelText(/Slack Incoming Webhook URL/), {
      target: { value: 'https://hooks.slack.com/services/x' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateChannel).toHaveBeenCalledWith('both', 'https://hooks.slack.com/services/x')
    )
    expect(await screen.findByText('Alert delivery updated.')).toBeInTheDocument()
  })

  it('surfaces a failure message when the update rejects', async () => {
    updateChannel.mockRejectedValue(new Error('nope'))
    const data = create(GetNotificationSettingsResponseSchema, { channelMode: 'email' })
    render(<NotificationChannelCard data={data} />)

    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('Failed to update alert delivery.')).toBeInTheDocument()
  })
})
