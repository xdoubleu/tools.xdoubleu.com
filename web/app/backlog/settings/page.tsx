'use client'

import { useState } from 'react'
import { useSettings, useSaveSettings } from '@/hooks/useSettings'
import { useImportBooks } from '@/hooks/useBacklog'
import { mutate } from 'swr'
import type { Integrations } from '@/lib/gen/settings/v1/settings_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function BacklogSettingsPage() {
  const { data, isLoading, error } = useSettings()
  const saveSettings = useSaveSettings()
  const importBooks = useImportBooks()

  const [steamApiKey, setSteamApiKey] = useState('')
  const [steamUserId, setSteamUserId] = useState('')
  const [hardcoverApiKey, setHardcoverApiKey] = useState('')
  const [saved, setSaved] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [initialized, setInitialized] = useState(false)
  const [importStatus, setImportStatus] = useState('')

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

  function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setImportStatus('Importing…')
    const reader = new FileReader()
    reader.onload = async (ev) => {
      const csvData = ev.target?.result
      if (typeof csvData !== 'string') return
      try {
        const res = await importBooks(csvData)
        setImportStatus(`Imported ${res.importedCount} book(s).`)
        await mutate('/backlog/books')
      } catch {
        setImportStatus('Import failed.')
      }
    }
    reader.readAsText(file)
    e.target.value = ''
  }

  return (
    <main className="mx-auto max-w-xl px-4 py-10">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Backlog', href: '/backlog' }, { label: 'Settings' }]}
      />
      <h1 className="mb-6 text-xl font-semibold text-fg">Settings</h1>

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
          <div className="space-y-3">
            <div>
              <label htmlFor="steam_api_key" className="mb-1 block text-sm text-subtle">
                API Key
              </label>
              <Input
                id="steam_api_key"
                type="password"
                autoComplete="off"
                value={steamApiKey}
                onChange={(e) => setSteamApiKey(e.target.value)}
              />
            </div>
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
            <Input
              id="hardcover_api_key"
              type="password"
              autoComplete="off"
              value={hardcoverApiKey}
              onChange={(e) => setHardcoverApiKey(e.target.value)}
            />
            <p className="mt-1 text-xs text-muted">
              Find your API key at hardcover.app → Settings → API.
            </p>
          </div>
        </section>

        <Button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </form>

      <section className="mt-10 border-t border-border pt-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
          Import books
        </h2>
        <p className="mb-3 text-xs text-muted">
          Import your library from a Goodreads or Hardcover CSV export.
        </p>
        <div className="flex items-center gap-2">
          <label className="inline-flex h-9 cursor-pointer items-center rounded-xl border border-border bg-surface px-3 text-sm text-fg transition-colors hover:bg-hover">
            Import CSV
            <input type="file" accept=".csv" onChange={handleImport} className="hidden" />
          </label>
          {importStatus && <span className="text-sm text-muted">{importStatus}</span>}
        </div>
      </section>
    </main>
  )
}
