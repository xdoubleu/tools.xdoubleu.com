'use client'

import { useState } from 'react'
import { useIntegrations, useSaveIntegrations } from '@/hooks/useGames'
import { mutate } from 'swr'
import type { GetIntegrationsResponse, Integrations } from '@/lib/gen/games/v1/games_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert } from '@/components/ui/alert'
import { Field } from '@/components/ui/field'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'

export default function GamesSettingsClient({
  initialData
}: {
  initialData?: GetIntegrationsResponse
}) {
  const { data, isLoading, error } = useIntegrations(initialData)
  const saveSettings = useSaveIntegrations()

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
    return <LoadingState className="py-16 text-center text-sm" />
  }

  if (error) {
    return <ErrorState what="settings" className="py-16 text-center text-sm" />
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setSaved(false)
    setSaveError('')
    try {
      const integrations: Integrations = {
        $typeName: 'games.v1.Integrations',
        steamUserId
      }
      await saveSettings(integrations)
      await mutate(swrKeys.gamesIntegrations)
      setSaved(true)
    } catch {
      setSaveError('Failed to save settings.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Games Settings"
        breadcrumb={[{ label: 'Games', href: '/dashboard/games' }, { label: 'Settings' }]}
      />

      {saved && (
        <Alert tone="success" className="mb-4">
          Settings saved successfully.
        </Alert>
      )}
      {saveError && (
        <Alert tone="danger" className="mb-4">
          {saveError}
        </Alert>
      )}

      <form onSubmit={handleSave} className="space-y-6">
        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Steam</h2>
          <Field label="Steam User ID" htmlFor="steam_user_id">
            <Input
              id="steam_user_id"
              type="text"
              inputMode="numeric"
              value={steamUserId}
              onChange={(e) => setSteamUserId(e.target.value)}
            />
          </Field>
        </section>

        <Button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </form>
    </PageContainer>
  )
}
