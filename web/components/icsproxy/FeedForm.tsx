'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useICSPreview, useSaveConfig } from '@/hooks/useICSProxy'
import { SaveConfigRequest } from '@/lib/gen/icsproxy/v1/proxy_pb'
import type { FilterConfig, EventInfo } from '@/lib/gen/icsproxy/v1/proxy_pb'

interface FeedFormProps {
  token?: string
  initialConfig?: FilterConfig
  initialEvents?: EventInfo[]
}

export default function FeedForm({ token, initialConfig, initialEvents }: FeedFormProps) {
  const router = useRouter()
  const saveConfig = useSaveConfig()

  const [sourceUrl, setSourceUrl] = useState(initialConfig?.sourceUrl || '')
  const [fetchUrl, setFetchUrl] = useState(initialConfig?.sourceUrl || '')
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

  const toggleHideEvent = (uid: string) => {
    setHideEventUids((prev) => {
      const next = new Set(prev)
      if (next.has(uid)) next.delete(uid)
      else next.add(uid)
      return next
    })
  }

  const toggleHoliday = (uid: string) => {
    setHolidayUids((prev) => {
      const next = new Set(prev)
      if (next.has(uid)) next.delete(uid)
      else next.add(uid)
      return next
    })
  }

  const toggleHideSeries = (seriesKey: string) => {
    setHideSeries((prev) => {
      const next = new Set(prev)
      if (next.has(seriesKey)) next.delete(seriesKey)
      else next.add(seriesKey)
      return next
    })
  }

  const handleFetch = () => {
    if (sourceUrl.trim()) setFetchUrl(sourceUrl.trim())
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSubmitting(true)
    setSubmitError(null)
    try {
      const result = await saveConfig(
        new SaveConfigRequest({
          token: token || '',
          sourceUrl: fetchUrl || sourceUrl,
          hideEventUids: Array.from(hideEventUids),
          holidayUids: Array.from(holidayUids),
          hideSeries: Array.from(hideSeries)
        })
      )
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
      <div>
        <label className="block text-sm font-medium text-subtle mb-1">Source Calendar URL</label>
        <div className="flex gap-2">
          <input
            type="url"
            value={sourceUrl}
            onChange={(e) => setSourceUrl(e.target.value)}
            required
            placeholder="https://example.com/calendar.ics"
            className="flex-1 px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            type="button"
            onClick={handleFetch}
            className="px-4 py-2 bg-subtle text-bg rounded hover:bg-fg text-sm"
          >
            Preview
          </button>
        </div>
      </div>

      {previewLoading && <p className="text-sm text-muted">Loading events...</p>}
      {previewError && (
        <p className="text-sm text-red-600">Failed to load events: {previewError.message}</p>
      )}

      {events.length > 0 && (
        <div>
          <p className="text-sm font-medium text-subtle mb-2">{events.length} events</p>
          <div className="overflow-x-auto rounded border border-border">
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
                    <td className="px-3 py-2">{event.summary || event.uid}</td>
                    <td className="px-3 py-2 text-muted">{event.startNice || event.startRaw}</td>
                    <td className="px-3 py-2 text-muted">{event.endNice || event.endRaw}</td>
                    <td className="px-3 py-2 text-center">{event.rrule ? 'Yes' : '—'}</td>
                    <td className="px-3 py-2 text-center">
                      <input
                        type="checkbox"
                        checked={hideEventUids.has(event.uid)}
                        onChange={() => toggleHideEvent(event.uid)}
                        className="accent-blue-600"
                      />
                    </td>
                    <td className="px-3 py-2 text-center">
                      <input
                        type="checkbox"
                        checked={holidayUids.has(event.uid)}
                        onChange={() => toggleHoliday(event.uid)}
                        className="accent-blue-600"
                      />
                    </td>
                    <td className="px-3 py-2 text-center">
                      {event.seriesKey ? (
                        <input
                          type="checkbox"
                          checked={hideSeries.has(event.seriesKey)}
                          onChange={() => toggleHideSeries(event.seriesKey)}
                          className="accent-blue-600"
                        />
                      ) : (
                        '—'
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {submitError && <p className="text-sm text-red-600">{submitError}</p>}

      <div className="flex gap-2">
        <button
          type="submit"
          disabled={isSubmitting}
          className="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
        >
          {isSubmitting ? 'Saving...' : 'Save Filter'}
        </button>
        <button
          type="button"
          onClick={() => router.push('/icsproxy')}
          className="flex-1 px-4 py-2 bg-subtle text-bg rounded hover:bg-fg"
        >
          Cancel
        </button>
      </div>
    </form>
  )
}
