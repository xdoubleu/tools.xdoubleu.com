'use client'

import { useNotificationSettings } from '@/hooks/useMonitoring'
import NotificationSettingsCard from './NotificationSettingsCard'

export default function NotificationSettingsClient() {
  const notificationSettings = useNotificationSettings()
  return <NotificationSettingsCard data={notificationSettings.data} />
}
