'use client'

import { useTodoSettings } from '@/hooks/useTodoSettings'
import { SettingsWorkspaces } from '@/components/todos/settings/SettingsWorkspaces'
import { SettingsSections } from '@/components/todos/settings/SettingsSections'
import { SettingsLabels } from '@/components/todos/settings/SettingsLabels'
import { SettingsURLPatterns } from '@/components/todos/settings/SettingsURLPatterns'
import { SettingsPolicies } from '@/components/todos/settings/SettingsPolicies'
import { SettingsArchive } from '@/components/todos/settings/SettingsArchive'

export default function TodoSettingsPage() {
  const { data, isLoading, error, mutate } = useTodoSettings()

  if (isLoading) {
    return <p className="py-16 text-center text-sm text-muted">Loading…</p>
  }

  if (error || !data) {
    return <p className="py-16 text-center text-sm text-danger">Failed to load settings.</p>
  }

  return (
    <div className="mx-auto max-w-2xl space-y-10">
      <h1 className="text-xl font-semibold text-fg">Settings</h1>
      <SettingsWorkspaces data={data} mutate={mutate} />
      <SettingsSections data={data} mutate={mutate} />
      <SettingsLabels data={data} mutate={mutate} />
      <SettingsURLPatterns data={data} mutate={mutate} />
      <SettingsArchive data={data} mutate={mutate} />
      <SettingsPolicies data={data} mutate={mutate} />
    </div>
  )
}
