'use client'

import { useState } from 'react'
import type { Recommendation } from '@/lib/gen/taste/v1/taste_pb'
import { useTasteRecommendations } from '@/hooks/useTaste'
import { DOMAINS, TRACKS, domainLabel, hostLabel, type Track } from '@/lib/taste/format'
import { cn } from '@/lib/cn'
import { formatDateTime } from '@/lib/dates'
import PickArtwork from '@/components/taste/PickArtwork'
import { Alert } from '@/components/ui/alert'
import { Card } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { SectionCard } from '@/components/ui/section-card'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { Select } from '@/components/ui/select'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

const linkClass = 'inline-flex min-h-11 min-w-11 items-center underline'

function PickRow({ pick }: { pick: Recommendation }) {
  const meta = [domainLabel(pick.domain), pick.creator, pick.year ? String(pick.year) : '']
  return (
    <Card variant="inset" className="flex items-start gap-3">
      <PickArtwork url={pick.imageUrl} title={pick.title} />
      <div className="min-w-0 flex-1">
        <p className="break-words font-medium">{pick.title}</p>
        <p className="break-words text-xs text-muted">{meta.filter(Boolean).join(' · ')}</p>
        <p className="mt-1 break-words text-sm text-subtle">{pick.why}</p>
        <div className="flex flex-wrap gap-x-4 text-sm">
          {pick.url && (
            <a href={pick.url} target="_blank" rel="noopener noreferrer" className={linkClass}>
              Open
            </a>
          )}
          {pick.sources.slice(0, 3).map((s, i) => (
            <a
              key={`${i}:${s.url}`}
              href={s.url}
              target="_blank"
              rel="noopener noreferrer"
              className={cn(linkClass, 'break-all')}
            >
              via {hostLabel(s.url)}
            </a>
          ))}
        </div>
      </div>
    </Card>
  )
}

function PickList({ domain, track }: { domain: string; track: Track }) {
  const { data, error, isLoading } = useTasteRecommendations(domain, track)

  if (isLoading && !data) return <LoadingState label="recommendations" />
  if (error && !data) return <ErrorState what="recommendations" />
  if (!data) return null

  return (
    <div className="space-y-3">
      {data.lastError && <Alert tone="warn">{data.lastError}</Alert>}
      {data.recommendations.length === 0 ? (
        <EmptyState>
          No picks yet. They are written after your profile, weekly or when you refresh it.
        </EmptyState>
      ) : (
        <ul className="space-y-2" aria-label="Recommendations">
          {data.recommendations.map((r) => (
            <li key={r.id}>
              <PickRow pick={r} />
            </li>
          ))}
        </ul>
      )}
      {data.generatedAt && (
        <p className="text-xs text-muted" suppressHydrationWarning>
          Picked {formatDateTime(data.generatedAt)}, each checked against its catalog.
        </p>
      )}
    </div>
  )
}

/** Picks per track and domain, written by simulated taste neighbours. */
export default function TasteRecommendations() {
  const [track, setTrack] = useState<Track>('more')
  const [domain, setDomain] = useState('')

  return (
    <SectionCard title="Recommendations">
      <div className="space-y-3">
        <SegmentedTabs
          aria-label="Track"
          value={track}
          options={TRACKS.map((t) => ({ value: t.value, label: t.label }))}
          onChange={setTrack}
        />
        <Field label="Domain" htmlFor="taste-pick-domain" className="w-44">
          <Select id="taste-pick-domain" value={domain} onChange={(e) => setDomain(e.target.value)}>
            <option value="">Everything</option>
            {DOMAINS.map((d) => (
              <option key={d.id} value={d.id}>
                {d.label}
              </option>
            ))}
          </Select>
        </Field>
        <PickList domain={domain} track={track} />
      </div>
    </SectionCard>
  )
}
