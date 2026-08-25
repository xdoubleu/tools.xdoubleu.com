'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { useNotificationSettings } from '@/hooks/useMonitoring'
import NotificationToggleList from '@/components/notifications/NotificationToggleList'

// feeds only owns unhealthy_feeds — sentry_issues/failing_dependency_prs are
// surfaced from the monitoring app instead (issue #1228).
const FEEDS_SOURCE_KEYS = ['unhealthy_feeds']

export default function FeedsNotificationSettingsCard() {
  const notificationSettings = useNotificationSettings()

  return (
    <Card>
      <CardHeader>
        <CardTitle>Email notifications</CardTitle>
        <CardDescription>Whether broken feeds are allowed to email an admin.</CardDescription>
      </CardHeader>
      <CardContent>
        <NotificationToggleList data={notificationSettings.data} sourceKeys={FEEDS_SOURCE_KEYS} />
      </CardContent>
    </Card>
  )
}
