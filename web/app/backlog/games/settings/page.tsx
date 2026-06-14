'use client'

import { useState } from 'react'
import { useSettings, useSaveSettings } from '@/hooks/useSettings'
import { mutate } from 'swr'
import type { Integrations } from '@/lib/gen/settings/v1/settings_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function BacklogGamesSettingsPage() {
  const { data, isLoading, error } = useSettings()
  const saveSettings = useSaveSettings()

  const [steamUserId, setSteamUserId] = useState('')
  const [saved, setSaved] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (!isLoading && data?.integrations && !initialized) {
    setSteamUserId(data.integrations.steamUserId)
    setInitialized(true)
  }

  if (isLoading) {
    return <p className="py-16 text-center text-sm text-muted">Loading…</p>
  }

  if (error) {
    return <p className="py-16 text-center text-sm text-danger">Failed to load settings.</p>
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setSaved(false)
    setSaveError('')
    try {
      const integrations: Integrations = {
        $typeName: 'settings.v1.Integrations',
        steamUserId
      }
      await saveSettings(integrations)
      await mutate('/settings')
      setSaved(true)
    } catch {
      setSaveError('Failed to save settings.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <main className="mx-auto max-w-xl px-4 py-10">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Games', href: '/backlog/games' }, { label: 'Settings' }]}
      />
      <h1 className="mb-6 text-xl font-semibold text-fg">Games Settings</h1>

      {saved && (
        <div className="mb-4 rounded-xl border border-success/30 bg-success/10 px-4 py-2 text-sm text-success">
          Settings saved successfully.
        </div>
      )}
      {saveError && (
        <div className="mb-4 rounded-xl border border-danger/30 bg-danger/10 px-4 py-2 text-sm text-danger">
          {saveError}
        </div>
      )}

      <form onSubmit={handleSave} className="space-y-6">
        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Steam</h2>
          <div>
            <label htmlFor="steam_user_id" className="mb-1 block text-sm text-subtle">
              Steam User ID
            </label>
            <Input
              id="steam_user_id"
              type="text"
              value={steamUserId}
              onChange={(e) => setSteamUserId(e.target.value)}
            />
          </div>
        </section>

        <Button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </form>
    </main>
  )
}
