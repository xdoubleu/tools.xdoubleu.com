'use client'

import { useState } from 'react'
import { ConnectError } from '@connectrpc/connect'
import type { ProfileSections } from '@/lib/gen/taste/v1/taste_pb'
import { useRefreshTasteProfile, useTasteProfile } from '@/hooks/useTaste'
import { formatDateTime } from '@/lib/dates'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { SectionCard } from '@/components/ui/section-card'
import { ErrorState, LoadingState } from '@/components/ui/states'

type ListKey = 'know' | 'like' | 'drifting' | 'bounce'

const SECTIONS: { key: ListKey; title: string }[] = [
  { key: 'know', title: 'What you know' },
  { key: 'like', title: 'What you like' },
  { key: 'drifting', title: 'Drifting toward' },
  { key: 'bounce', title: 'Bounce off' }
]

function Sections({ sections }: { sections: ProfileSections }) {
  return (
    <div className="space-y-4">
      <p className="break-words">{sections.summary}</p>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {SECTIONS.filter((s) => sections[s.key].length > 0).map((s) => (
          <section key={s.key}>
            <h3 className="text-sm font-semibold">{s.title}</h3>
            <ul className="mt-1 list-disc space-y-1 pl-5 text-sm text-subtle">
              {sections[s.key].map((bullet, i) => (
                <li key={`${i}:${bullet}`} className="break-words">
                  {bullet}
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  )
}

function RefreshButton({ waitUntil, pending }: { waitUntil: string; pending: boolean }) {
  const refresh = useRefreshTasteProfile()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function onClick() {
    setBusy(true)
    setError('')
    try {
      await refresh()
    } catch (err) {
      setError(ConnectError.from(err).rawMessage)
    } finally {
      setBusy(false)
    }
  }

  const label = pending || busy ? 'Refreshing…' : 'Refresh'
  return (
    <div className="flex flex-col items-end gap-1">
      <Button
        size="sm"
        variant="secondary"
        onClick={onClick}
        disabled={pending || busy || waitUntil !== ''}
      >
        {label}
      </Button>
      {waitUntil !== '' && !pending && (
        <p className="text-xs text-muted" suppressHydrationWarning>
          Again after {formatDateTime(waitUntil)}
        </p>
      )}
      {error && (
        <p role="alert" className="text-xs text-danger">
          {error}
        </p>
      )}
    </div>
  )
}

/** The LLM-written taste profile with its refresh control. */
export default function TasteProfile() {
  const { data, error, isLoading } = useTasteProfile()

  if (isLoading && !data) return <LoadingState label="profile" />
  if (error && !data) return <ErrorState what="profile" />
  if (!data) return null

  if (!data.configured) {
    return (
      <SectionCard title="Your taste profile">
        <p className="text-sm text-muted">
          Profiles are written by an LLM and need an OpenRouter API key on the server.
        </p>
      </SectionCard>
    )
  }

  const written = data.generatedAt
    ? `Written ${formatDateTime(data.generatedAt)} by ${data.model}`
    : undefined
  return (
    <SectionCard
      title="Your taste profile"
      description={written && <span suppressHydrationWarning>{written}</span>}
      action={<RefreshButton waitUntil={data.nextRefreshAt} pending={data.pending} />}
    >
      <div className="space-y-4">
        {data.lastError && <Alert tone="warn">{data.lastError}</Alert>}
        {data.pending && <p className="text-sm text-muted">Writing your profile…</p>}
        {data.sections ? (
          <Sections sections={data.sections} />
        ) : (
          !data.pending && (
            <p className="text-sm text-muted">
              No profile yet. One is written within the hour and weekly after that; refresh to queue
              it now.
            </p>
          )
        )}
      </div>
    </SectionCard>
  )
}
