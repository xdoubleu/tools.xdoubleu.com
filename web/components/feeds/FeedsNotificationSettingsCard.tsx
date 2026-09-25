'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { useNotificationSettings } from '@/hooks/useMonitoring'
import NotificationToggleList from '@/components/notifications/NotificationToggleList'

// The other sources live on the monitoring page.
const FEEDS_SOURCE_KEYS = ['unhealthy_feeds', 'open_feed_items']

export default function FeedsNotificationSettingsCard() {
  const notificationSettings = useNotificationSettings()

  return (
    <Card>
      <CardHeader>
        <CardTitle>Email notifications</CardTitle>
        <CardDescription>
          Whether broken feeds or unread items are allowed to email an admin.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <NotificationToggleList data={notificationSettings.data} sourceKeys={FEEDS_SOURCE_KEYS} />
      </CardContent>
    </Card>
  )
}
