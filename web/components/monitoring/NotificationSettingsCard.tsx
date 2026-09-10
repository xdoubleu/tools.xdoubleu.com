'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import type { GetNotificationSettingsResponse } from '@/lib/gen/observability/v1/observability_pb'
import NotificationToggleList from '@/components/notifications/NotificationToggleList'

// Monitoring owns sentry_issues/failing_dependency_prs/security_alerts/
// orphaned_storage — unhealthy_feeds is surfaced from the feeds app instead
// (issue #1228). The host/CI/R2 sources moved to Grafana + Prometheus alert
// rules (issue #1468), and the per-class slow-transaction latency alerts
// followed (issue #1528, ADR-0011) — Grafana now evaluates real Prometheus
// histograms and routes through its own SMTP contact point, so those
// toggles no longer exist here.
const MONITORING_SOURCE_KEYS = [
  'sentry_issues',
  'failing_dependency_prs',
  'security_alerts',
  'orphaned_storage'
]

export default function NotificationSettingsCard({
  data
}: {
  data?: GetNotificationSettingsResponse
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Email notifications</CardTitle>
        <CardDescription>
          {data?.adminEmail
            ? `Which monitoring sources are allowed to email ${data.adminEmail}.`
            : 'Which monitoring sources are allowed to email an admin.'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <NotificationToggleList data={data} sourceKeys={MONITORING_SOURCE_KEYS} />
      </CardContent>
    </Card>
  )
}
