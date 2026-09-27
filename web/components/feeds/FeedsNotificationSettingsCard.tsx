'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { useCurrentUser } from '@/hooks/useAuth'
import { useNotificationSettings } from '@/hooks/useMonitoring'
import NotificationToggleList from '@/components/notifications/NotificationToggleList'

// The other sources live on the monitoring page.
const FEEDS_SOURCE_KEYS = ['unhealthy_feeds', 'open_feed_items']

// Admin-only: the toggles are global and only admins may change them.
export default function FeedsNotificationSettingsCard() {
  const { data: currentUser } = useCurrentUser()
  const notificationSettings = useNotificationSettings()

  if (currentUser?.role !== 'admin') return null

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
