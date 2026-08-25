'use client'

import { useState } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import type { GetNotificationSettingsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { useUpdateNotificationSettings } from '@/hooks/useMonitoring'

const SOURCE_LABELS: Record<string, string> = {
  sentry_issues: 'Sentry issues',
  failing_dependency_prs: 'Failing dependency PRs',
  unhealthy_feeds: 'Unhealthy feeds'
}

const SOURCE_DESCRIPTIONS: Record<string, string> = {
  sentry_issues: 'Emails an admin the first time a new unresolved Sentry issue appears.',
  failing_dependency_prs:
    'Emails an admin the first time a dependency (Renovate) pull request fails CI.',
  unhealthy_feeds: 'Includes feeds failing to poll in the weekly digest email.'
}

function sourceLabel(sourceKey: string): string {
  return SOURCE_LABELS[sourceKey] ?? sourceKey
}

function sourceDescription(sourceKey: string): string {
  return SOURCE_DESCRIPTIONS[sourceKey] ?? ''
}

export default function NotificationSettingsCard({
  data
}: {
  data?: GetNotificationSettingsResponse
}) {
  const updateNotificationSettings = useUpdateNotificationSettings()
  const [pendingKey, setPendingKey] = useState<string | null>(null)

  async function handleToggle(sourceKey: string, enabled: boolean) {
    setPendingKey(sourceKey)
    try {
      await updateNotificationSettings(sourceKey, enabled)
    } finally {
      setPendingKey(null)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Email notifications</CardTitle>
        <CardDescription>Which monitoring sources are allowed to email an admin.</CardDescription>
      </CardHeader>
      <CardContent>
        {!data ? (
          <p className="py-8 text-center text-sm text-muted">Loading…</p>
        ) : (
          <ul className="space-y-3">
            {data.settings.map((setting) => (
              <li key={setting.sourceKey} className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-medium text-fg">{sourceLabel(setting.sourceKey)}</p>
                  <p className="text-xs text-muted">{sourceDescription(setting.sourceKey)}</p>
                </div>
                <Checkbox
                  checked={setting.enabled}
                  disabled={pendingKey === setting.sourceKey}
                  onChange={(e) => handleToggle(setting.sourceKey, e.target.checked)}
                  aria-label={`Email notifications for ${sourceLabel(setting.sourceKey)}`}
                />
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
