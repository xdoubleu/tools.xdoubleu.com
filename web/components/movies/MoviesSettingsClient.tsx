'use client'

import { useState } from 'react'
import Image from 'next/image'
import { useAvailableProviders, useMovieSettings, useMoviesActions } from '@/hooks/useMovies'
import { logoUrl } from '@/lib/movies/format'
import TmdbAttribution from '@/components/movies/TmdbAttribution'
import { Alert } from '@/components/ui/alert'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { TogglePill } from '@/components/ui/toggle-pill'

/** Pick the Belgian streaming services you subscribe to. */
export default function MoviesSettingsClient() {
  const settings = useMovieSettings()
  const providers = useAvailableProviders()
  const { setServices } = useMoviesActions()
  const [saving, setSaving] = useState(false)
  const [failed, setFailed] = useState(false)

  const breadcrumb = [{ label: 'Movies & Series', href: '/movies' }, { label: 'Settings' }]
  const picked = settings.data?.providerIds ?? []

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

  let body
  if ((settings.isLoading && !settings.data) || (providers.isLoading && !providers.data)) {
    body = <LoadingState label="services" />
  } else if (settings.error || providers.error || !providers.data) {
    body = <ErrorState what="services" />
  } else {
    body = (
      <ul aria-label="Streaming services" className="flex flex-wrap gap-2">
        {providers.data.providers.map((p) => (
          <li key={p.id}>
            <TogglePill
              active={picked.includes(p.id)}
              disabled={saving}
              onClick={() => void toggle(p.id)}
              label={
                <span className="flex items-center gap-2">
                  {p.logoPath && (
                    <Image
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
    )
  }

  return (
    <PageContainer>
      <PageHeader title="Movies & Series settings" breadcrumb={breadcrumb} />
      <p className="mb-4 text-sm text-muted">
        Pick your streaming services. Titles you can watch on them without renting or buying are
        marked in your backlog and can be filtered.
      </p>
      {failed && (
        <Alert tone="danger" className="mb-4">
          Couldn&apos;t save. Try again.
        </Alert>
      )}
      {body}
      <TmdbAttribution />
    </PageContainer>
  )
}
