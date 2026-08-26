'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import type { GetNotificationSettingsResponse } from '@/lib/gen/observability/v1/observability_pb'
import NotificationToggleList from '@/components/notifications/NotificationToggleList'

// Monitoring owns sentry_issues/failing_dependency_prs/failing_main_ci/
// security_alerts — unhealthy_feeds is surfaced from the feeds app instead
// (issue #1228).
const MONITORING_SOURCE_KEYS = [
  'sentry_issues',
  'failing_dependency_prs',
  'failing_main_ci',
  'security_alerts'
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
