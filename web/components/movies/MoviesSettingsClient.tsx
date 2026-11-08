import { useState } from 'react'
import type { Provider } from '@/lib/gen/movies/v1/movies_pb'
import { useAvailableProviders, useMovieSettings, useMoviesActions } from '@/hooks/useMovies'
import { logoUrl } from '@/lib/movies/format'
import TmdbAttribution from '@/components/movies/TmdbAttribution'
import { Alert } from '@/components/ui/alert'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { SettingsSection, SettingsSections } from '@/components/ui/settings-section'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { TogglePill } from '@/components/ui/toggle-pill'

function ProviderPicker({ picked, providers }: { picked: bigint[]; providers: Provider[] }) {
  const { setServices } = useMoviesActions()
  const [saving, setSaving] = useState(false)
  const [failed, setFailed] = useState(false)

  const toggle = async (id: bigint) => {
    setSaving(true)
    setFailed(false)
    try {
      await setServices(picked.includes(id) ? picked.filter((p) => p !== id) : [...picked, id])
    } catch {
      setFailed(true)
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      {failed && (
        <Alert tone="danger" className="mb-4">
          Couldn&apos;t save. Try again.
        </Alert>
      )}
      <ul aria-label="Streaming services" className="flex flex-wrap gap-2">
        {providers.map((p) => (
          <li key={p.id}>
            <TogglePill
              active={picked.includes(p.id)}
              disabled={saving}
              onClick={() => void toggle(p.id)}
              label={
                <span className="flex items-center gap-2">
                  {p.logoPath && (
                    <img
                      src={logoUrl(p.logoPath)}
                      alt=""
                      width={24}
                      height={24}
                      className="rounded"
                      loading="lazy"
                    />
                  )}
                  {p.name}
                </span>
              }
            />
          </li>
        ))}
      </ul>
    </>
  )
}

/** Pick the Belgian streaming services you subscribe to. */
export default function MoviesSettingsClient() {
  const settings = useMovieSettings()
  const providers = useAvailableProviders()

  let body
  // A failed revalidation keeps showing the loaded picker.
  if (!settings.data || !providers.data) {
    body =
      settings.error || providers.error ? (
        <ErrorState what="services" placement="inline" />
      ) : (
        <LoadingState label="services" placement="inline" />
      )
  } else {
    body = (
      <ProviderPicker picked={settings.data.providerIds} providers={providers.data.providers} />
    )
  }

  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Movies & series settings"
        breadcrumb={[{ label: 'Movies & series', href: '/movies' }, { label: 'Settings' }]}
      />
      <SettingsSections>
        <SettingsSection
          title="Streaming services"
          description="Titles you can watch on them without renting or buying are marked in your backlog and can be filtered."
        >
          {body}
        </SettingsSection>
      </SettingsSections>
      <TmdbAttribution />
    </PageContainer>
  )
}
