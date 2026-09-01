'use client'

import { useState } from 'react'
import { Checkbox } from '@/components/ui/checkbox'
import type { GetNotificationSettingsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { useUpdateNotificationSettings } from '@/hooks/useMonitoring'

const SOURCE_LABELS: Record<string, string> = {
  sentry_issues: 'Sentry issues',
  failing_dependency_prs: 'Failing dependency PRs',
  unhealthy_feeds: 'Unhealthy feeds',
  failing_main_ci: 'Failing PRs on main',
  security_alerts: 'Security alerts',
  orphaned_storage: 'Orphaned storage',
  host_cpu_high: 'Host CPU high',
  host_memory_high: 'Host memory high',
  host_disk_high: 'Host disk high',
  r2_usage_high: 'R2 storage usage high',
  ci_duration_high: 'CI workflow duration high',
  slow_transaction_http_high: 'Slow HTTP handlers',
  slow_transaction_job_high: 'Slow background jobs',
  slow_transaction_frontend_high: 'Slow frontend spans'
}

const SOURCE_DESCRIPTIONS: Record<string, string> = {
  sentry_issues: 'Emails an admin the first time a new unresolved Sentry issue appears.',
  failing_dependency_prs:
    'Emails an admin the first time a dependency (Renovate) pull request fails CI.',
  unhealthy_feeds: 'Includes feeds failing to poll in the weekly digest email.',
  failing_main_ci: 'Emails an admin the first time a CI run fails on the main branch.',
  security_alerts:
    'Emails an admin the first time a Dependabot, code-scanning, or secret-scanning alert appears.',
  orphaned_storage:
    'Emails an admin the first time an orphaned R2 storage object is detected by the daily scan.',
  host_cpu_high:
    'Emails an admin when host CPU usage stays above threshold for 15 minutes, and again on recovery.',
  host_memory_high:
    'Emails an admin when host memory usage stays above threshold for 15 minutes, and again on recovery.',
  host_disk_high:
    'Emails an admin when host disk usage goes above threshold, and again on recovery.',
  r2_usage_high:
    'Emails an admin when total R2 storage usage goes above threshold, and again on recovery.',
  ci_duration_high:
    "Emails an admin when a workflow's CI duration (p95) goes above threshold, and again on recovery.",
  slow_transaction_http_high:
    "Emails an admin when an HTTP handler's p95 duration goes above threshold, and again on recovery.",
  slow_transaction_job_high:
    "Emails an admin when a background job's p95 duration goes above threshold, and again on recovery.",
  slow_transaction_frontend_high:
    "Emails an admin when a frontend span's p95 duration goes above threshold, and again on recovery."
}

function sourceLabel(sourceKey: string): string {
  return SOURCE_LABELS[sourceKey] ?? sourceKey
}

function sourceDescription(sourceKey: string): string {
  return SOURCE_DESCRIPTIONS[sourceKey] ?? ''
}

// NotificationToggleList renders one checkbox per source_key in sourceKeys,
// shared by the monitoring and feeds notification-settings pages (issue
// #1228) — each page passes its own allowlist of source keys so a source
// added directly to global.notification_settings doesn't silently leak into
// the wrong app's page.
export default function NotificationToggleList({
  data,
  sourceKeys
}: {
  data?: GetNotificationSettingsResponse
  sourceKeys: string[]
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

  if (!data) {
    return <p className="py-8 text-center text-sm text-muted">Loading…</p>
  }

  const settings = data.settings.filter((s) => sourceKeys.includes(s.sourceKey))

  return (
    <ul className="space-y-3">
      {settings.map((setting) => (
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
  )
}
