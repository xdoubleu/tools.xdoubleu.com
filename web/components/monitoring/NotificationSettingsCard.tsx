'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import type { GetNotificationSettingsResponse } from '@/lib/gen/observability/v1/observability_pb'
import NotificationToggleList from '@/components/notifications/NotificationToggleList'

// Monitoring owns sentry_issues/failing_dependency_prs/security_alerts/
// orphaned_storage, plus the slow-transaction threshold rules (issue
// #1310, kept per ADR-0011) — unhealthy_feeds is surfaced from the feeds
// app instead (issue #1228). The host/CI-duration/R2-usage/failing-main-CI
// sources moved to Grafana + Prometheus alert rules (issue #1468) and no
// longer exist here.
const MONITORING_SOURCE_KEYS = [
  'sentry_issues',
  'failing_dependency_prs',
  'security_alerts',
  'orphaned_storage',
  'slow_transaction_http_high',
  'slow_transaction_job_high',
  'slow_transaction_frontend_high'
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
