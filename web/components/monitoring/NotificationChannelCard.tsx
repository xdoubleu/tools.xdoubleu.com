'use client'

import { useState } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { useUpdateNotificationChannel } from '@/hooks/useMonitoring'
import type { GetNotificationSettingsResponse } from '@/lib/gen/observability/v1/observability_pb'

const MODE_OPTIONS = [
  { value: 'email', label: 'Email only' },
  { value: 'slack', label: 'Slack only' },
  { value: 'both', label: 'Email and Slack' }
]

// NotificationChannelCard is the global delivery switch for real-time and
// threshold alerts (issue #1482). The weekly digests always email regardless.
// The Slack webhook URL is write-only here — the server never returns it, only
// whether one is configured.
export default function NotificationChannelCard({
  data
}: {
  data?: GetNotificationSettingsResponse
}) {
  const updateChannel = useUpdateNotificationChannel()

  const [mode, setMode] = useState<string | null>(null)
  const [webhookUrl, setWebhookUrl] = useState('')
  const [pending, setPending] = useState(false)
  const [message, setMessage] = useState<{ tone: 'success' | 'danger'; text: string } | null>(null)

  if (!data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Alert delivery</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted">Loading…</p>
        </CardContent>
      </Card>
    )
  }

  const selectedMode = mode ?? data.channelMode ?? 'email'
  const slackSelected = selectedMode === 'slack' || selectedMode === 'both'

  async function onSave() {
    setPending(true)
    setMessage(null)
    try {
      // Only send the webhook URL when the admin actually typed one, so an
      // untouched field never clears the stored value.
      await updateChannel(selectedMode, webhookUrl !== '' ? webhookUrl : undefined)
      setWebhookUrl('')
      setMode(null)
      setMessage({ tone: 'success', text: 'Alert delivery updated.' })
    } catch {
      setMessage({ tone: 'danger', text: 'Failed to update alert delivery.' })
    } finally {
      setPending(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Alert delivery</CardTitle>
        <CardDescription>
          Where real-time and threshold alerts are sent. Weekly digests always go to email.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <RadioGroup name="channel-mode" value={selectedMode} onChange={(value) => setMode(value)}>
          {MODE_OPTIONS.map((option) => (
            <RadioGroupItem key={option.value} value={option.value} label={option.label} />
          ))}
        </RadioGroup>

        {slackSelected && (
          <div className="space-y-1">
            <Label htmlFor="slack-webhook-url">Slack Incoming Webhook URL</Label>
            <Input
              id="slack-webhook-url"
              type="password"
              autoComplete="off"
              placeholder={
                data.slackWebhookConfigured
                  ? 'Configured — type a new URL to replace it'
                  : 'https://hooks.slack.com/services/…'
              }
              value={webhookUrl}
              onChange={(event) => setWebhookUrl(event.target.value)}
            />
            <p className="text-sm text-muted">
              {data.slackWebhookConfigured
                ? 'A webhook URL is configured.'
                : 'No webhook URL configured yet — alerts fall back to email until one is set.'}
            </p>
          </div>
        )}

        <div className="flex items-center gap-3">
          <Button onClick={onSave} disabled={pending}>
            {pending ? 'Saving…' : 'Save'}
          </Button>
          {message && (
            <p className={`text-sm ${message.tone === 'success' ? 'text-success' : 'text-danger'}`}>
              {message.text}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
