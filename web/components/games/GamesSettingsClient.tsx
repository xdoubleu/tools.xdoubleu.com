import { useState } from 'react'
import { useIntegrations, useSaveIntegrations } from '@/hooks/useGames'
import { mutate } from 'swr'
import type { Integrations } from '@/lib/gen/games/v1/games_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert } from '@/components/ui/alert'
import { Field } from '@/components/ui/field'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { SettingsSection, SettingsSections } from '@/components/ui/settings-section'
import { ErrorState, LoadingState } from '@/components/ui/states'

export default function GamesSettingsClient() {
  const { data, isLoading, error } = useIntegrations()
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
    return <LoadingState placement="page" />
  }

  if (error) {
    return <ErrorState what="settings" placement="page" />
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
        title="Games settings"
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
        <SettingsSections>
          <SettingsSection title="Steam">
            <Field label="Steam user ID" htmlFor="steam_user_id">
              <Input
                id="steam_user_id"
                type="text"
                inputMode="numeric"
                value={steamUserId}
                onChange={(e) => setSteamUserId(e.target.value)}
              />
            </Field>
          </SettingsSection>
        </SettingsSections>

        <Button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </form>
    </PageContainer>
  )
}
