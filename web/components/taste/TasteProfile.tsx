'use client'

import type { ProfileSections } from '@/lib/gen/taste/v1/taste_pb'
import { useRefreshTasteProfile, useTasteProfile } from '@/hooks/useTaste'
import { useSharedList } from '@/hooks/useSharedList'
import { formatDateTime } from '@/lib/dates'
import RefreshButton from '@/components/taste/RefreshButton'
import { Alert } from '@/components/ui/alert'
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

/** The LLM-written taste profile with its refresh control. */
export default function TasteProfile() {
  const shared = useSharedList()
  const { data, error, isLoading } = useTasteProfile()
  const refresh = useRefreshTasteProfile()

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

  return (
    <SectionCard
      title={shared ? "The family's taste profile" : 'Your taste profile'}
      action={
        <RefreshButton refresh={refresh} waitUntil={data.nextRefreshAt} pending={data.pending} />
      }
    >
      <div className="space-y-4">
        {data.lastError && <Alert tone="warn">{data.lastError}</Alert>}
        {data.pending && <p className="text-sm text-muted">Writing your profile…</p>}
        {data.sections ? (
          <>
            <Sections sections={data.sections} />
            <p className="break-all text-xs text-muted" suppressHydrationWarning>
              Written {formatDateTime(data.generatedAt)} by {data.model}
            </p>
          </>
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
