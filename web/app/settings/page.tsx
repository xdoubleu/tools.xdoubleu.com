'use client'

import { useState } from 'react'
import { useSettings, useSaveSettings } from '@/hooks/useSettings'
import { mutate } from 'swr'
import type { Integrations } from '@/lib/gen/settings/v1/settings_pb'

export default function SettingsPage() {
  const { data, isLoading, error } = useSettings()
  const saveSettings = useSaveSettings()

  const [steamApiKey, setSteamApiKey] = useState('')
  const [steamUserId, setSteamUserId] = useState('')
  const [hardcoverApiKey, setHardcoverApiKey] = useState('')
  const [saved, setSaved] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [initialized, setInitialized] = useState(false)

  if (!isLoading && data?.integrations && !initialized) {
    setSteamApiKey(data.integrations.steamApiKey)
    setSteamUserId(data.integrations.steamUserId)
    setHardcoverApiKey(data.integrations.hardcoverApiKey)
    setInitialized(true)
  }

  if (isLoading) {
    return <p className="py-16 text-center text-sm text-muted">Loading…</p>
  }

  if (error) {
    return <p className="py-16 text-center text-sm text-red-500">Failed to load settings.</p>
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setSaved(false)
    setSaveError('')
    try {
      const integrations: Integrations = {
        $typeName: 'settings.v1.Integrations',
        steamApiKey,
        steamUserId,
        hardcoverApiKey
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
      <div className="mb-6 flex items-center gap-3">
        <nav className="text-sm text-muted">
          <span className="font-semibold uppercase tracking-wide text-xs text-muted">Backlog</span>
        </nav>
      </div>

      <h1 className="mb-6 text-xl font-semibold text-fg">Integrations</h1>

      {saved && (
        <div className="mb-4 rounded border border-green-300 bg-green-50 px-4 py-2 text-sm text-green-800 dark:border-green-700 dark:bg-green-950 dark:text-green-200">
          Settings saved successfully.
        </div>
      )}
      {saveError && (
        <div className="mb-4 rounded border border-red-300 bg-red-50 px-4 py-2 text-sm text-red-800 dark:border-red-700 dark:bg-red-950 dark:text-red-200">
          {saveError}
        </div>
      )}

      <form onSubmit={handleSave} className="space-y-6">
        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Steam</h2>
          <div className="space-y-3">
            <div>
              <label htmlFor="steam_api_key" className="mb-1 block text-sm text-subtle">
                API Key
              </label>
              <input
                id="steam_api_key"
                type="password"
                autoComplete="off"
                value={steamApiKey}
                onChange={(e) => setSteamApiKey(e.target.value)}
                className="w-full rounded border border-border bg-surface px-3 py-2 text-sm text-fg placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-fg"
              />
            </div>
            <div>
              <label htmlFor="steam_user_id" className="mb-1 block text-sm text-subtle">
                Steam User ID
              </label>
              <input
                id="steam_user_id"
                type="text"
                value={steamUserId}
                onChange={(e) => setSteamUserId(e.target.value)}
                className="w-full rounded border border-border bg-surface px-3 py-2 text-sm text-fg placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-fg"
              />
            </div>
          </div>
        </section>

        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
            Hardcover
          </h2>
          <div>
            <label htmlFor="hardcover_api_key" className="mb-1 block text-sm text-subtle">
              API Key
            </label>
            <input
              id="hardcover_api_key"
              type="password"
              autoComplete="off"
              value={hardcoverApiKey}
              onChange={(e) => setHardcoverApiKey(e.target.value)}
              className="w-full rounded border border-border bg-surface px-3 py-2 text-sm text-fg placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-fg"
            />
            <p className="mt-1 text-xs text-muted">
              Find your API key at hardcover.app → Settings → API.
            </p>
          </div>
        </section>

        <button
          type="submit"
          disabled={saving}
          className="rounded bg-fg px-4 py-2 text-sm font-medium text-bg hover:opacity-80 disabled:opacity-50"
        >
          {saving ? 'Saving…' : 'Save'}
        </button>
      </form>
    </main>
  )
}
