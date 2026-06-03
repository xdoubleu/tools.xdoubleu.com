'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useICSPreview, useSaveConfig } from '@/hooks/useICSProxy'
import type { SaveConfigInput } from '@/hooks/useICSProxy'
import type { FilterConfig, EventInfo } from '@/lib/gen/icsproxy/v1/proxy_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface FeedFormProps {
  token?: string
  initialConfig?: FilterConfig
  initialEvents?: EventInfo[]
}

export default function FeedForm({ token, initialConfig, initialEvents }: FeedFormProps) {
  const router = useRouter()
  const saveConfig = useSaveConfig()

  const [sourceUrl, setSourceUrl] = useState(initialConfig?.sourceUrl || '')
  const [fetchUrl, setFetchUrl] = useState('')
  const [hideEventUids, setHideEventUids] = useState<Set<string>>(
    new Set(initialConfig?.hideEventUids || [])
  )
  const [holidayUids, setHolidayUids] = useState<Set<string>>(
    new Set(initialConfig?.holidayUids || [])
  )
  const [hideSeries, setHideSeries] = useState<Set<string>>(
    new Set(initialConfig?.hideSeries || [])
  )
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const { data: preview, isLoading: previewLoading, error: previewError } = useICSPreview(fetchUrl)

  const events: EventInfo[] = fetchUrl ? (preview?.events ?? []) : (initialEvents ?? [])

  const toggleSet = (setter: React.Dispatch<React.SetStateAction<Set<string>>>, key: string) => {
    setter((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSubmitting(true)
    setSubmitError(null)
    try {
      const req: SaveConfigInput = {
        token: token || '',
        sourceUrl: fetchUrl || sourceUrl,
        hideEventUids: Array.from(hideEventUids),
        holidayUids: Array.from(holidayUids),
        hideSeries: Array.from(hideSeries)
      }
      const result = await saveConfig(req)
      router.push('/icsproxy')
      void result
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Failed to save filter config.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <div className="space-y-1.5">
        <Label>Source Calendar URL</Label>
        <div className="flex gap-2">
          <Input
            type="url"
            value={sourceUrl}
            onChange={(e) => setSourceUrl(e.target.value)}
            required
            placeholder="https://example.com/calendar.ics"
            className="flex-1"
          />
          <Button
            type="button"
            variant="secondary"
            onClick={() => {
              if (sourceUrl.trim()) setFetchUrl(sourceUrl.trim())
            }}
          >
            Preview
          </Button>
        </div>
      </div>

      {previewLoading && <p className="text-sm text-muted">Loading events...</p>}
      {previewError && (
        <p className="text-sm text-danger">Failed to load events: {previewError.message}</p>
      )}

      {events.length > 0 && (
        <div>
          <p className="text-sm font-medium text-subtle mb-2">{events.length} events</p>
          <div className="overflow-x-auto rounded-2xl border border-border">
            <table className="w-full text-sm">
              <thead className="bg-surface text-subtle">
                <tr>
                  <th className="px-3 py-2 text-left font-medium">Summary</th>
                  <th className="px-3 py-2 text-left font-medium">Start</th>
                  <th className="px-3 py-2 text-left font-medium">End</th>
                  <th className="px-3 py-2 text-center font-medium">Recurring</th>
                  <th className="px-3 py-2 text-center font-medium">Hide</th>
                  <th className="px-3 py-2 text-center font-medium">Holiday</th>
                  <th className="px-3 py-2 text-center font-medium">Hide Series</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {events.map((event) => (
                  <tr key={event.uid} className="hover:bg-surface">
                    <td className="px-3 py-2 text-fg">{event.summary || event.uid}</td>
                    <td className="px-3 py-2 text-muted">{event.startNice || event.startRaw}</td>
                    <td className="px-3 py-2 text-muted">{event.endNice || event.endRaw}</td>
                    <td className="px-3 py-2 text-center text-muted">
                      {event.rrule ? 'Yes' : '—'}
                    </td>
                    <td className="px-3 py-2 text-center">
                      <input
                        type="checkbox"
                        checked={hideEventUids.has(event.uid)}
                        onChange={() => toggleSet(setHideEventUids, event.uid)}
                        className="accent-[rgb(var(--color-accent))]"
                      />
                    </td>
                    <td className="px-3 py-2 text-center">
                      <input
                        type="checkbox"
                        checked={holidayUids.has(event.uid)}
                        onChange={() => toggleSet(setHolidayUids, event.uid)}
                        className="accent-[rgb(var(--color-accent))]"
                      />
                    </td>
                    <td className="px-3 py-2 text-center">
                      {event.seriesKey ? (
                        <input
                          type="checkbox"
                          checked={hideSeries.has(event.seriesKey)}
                          onChange={() => toggleSet(setHideSeries, event.seriesKey)}
                          className="accent-[rgb(var(--color-accent))]"
                        />
                      ) : (
                        <span className="text-muted">—</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {submitError && <p className="text-sm text-danger">{submitError}</p>}

      <div className="flex gap-2">
        <Button type="submit" disabled={isSubmitting} className="flex-1">
          {isSubmitting ? 'Saving...' : 'Save Filter'}
        </Button>
        <Button
          type="button"
          variant="secondary"
          onClick={() => router.push('/icsproxy')}
          className="flex-1"
        >
          Cancel
        </Button>
      </div>
    </form>
  )
}
