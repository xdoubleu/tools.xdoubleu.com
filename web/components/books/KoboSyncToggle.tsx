'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useEnableKoboSync, useToggleTag, useKEPUBStatus } from '@/hooks/useBooks'

interface KoboSyncToggleProps {
  bookId: string
  enabled: boolean
  tags: string[]
  onChanged?: () => void
}

function kepubStatusLabel(status: string): string {
  if (status === 'converting') return 'Preparing for Kobo...'
  if (status === 'ready') return 'Ready to sync'
  if (status === 'failed') return 'Conversion failed'
  return ''
}

export default function KoboSyncToggle({ bookId, enabled, tags, onChanged }: KoboSyncToggleProps) {
  const [enabledState, setEnabledState] = useState(enabled)
  const [wantsPDF, setWantsPDF] = useState(tags.includes('kobo-format-pdf'))
  const [toggling, setToggling] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const enableKoboSync = useEnableKoboSync()
  const toggleTag = useToggleTag()

  const { data: statusData } = useKEPUBStatus(bookId)

  const hasEpub = statusData?.hasEpub ?? false
  const hasPdf = statusData?.hasPdf ?? false
  const kepubStatus = statusData?.kepubStatus ?? ''

  const canEnable = hasEpub || hasPdf

  const handleToggle = async () => {
    const wasEnabled = enabledState
    setEnabledState(!wasEnabled)
    setToggling(true)
    setError(null)
    try {
      if (wasEnabled) {
        await toggleTag(bookId, 'kobo-sync')
      } else {
        await enableKoboSync(bookId)
        mutate(['/books/kepub-status', bookId])
      }
      onChanged?.()
    } catch (err) {
      setEnabledState(wasEnabled)
      setError(err instanceof Error ? err.message : 'Failed to update Kobo sync.')
    } finally {
      setToggling(false)
    }
  }

  const handleFormatChange = async (sendPDF: boolean) => {
    // Optimistic flip — respond instantly like the sync checkbox does.
    setWantsPDF(sendPDF)
    setToggling(true)
    setError(null)
    try {
      if (sendPDF) {
        // Add kobo-format-pdf tag to serve raw PDF.
        await toggleTag(bookId, 'kobo-format-pdf')
      } else {
        // Remove kobo-format-pdf; re-trigger conversion so the KEPUB is ready.
        await toggleTag(bookId, 'kobo-format-pdf')
        await enableKoboSync(bookId)
        mutate(['/books/kepub-status', bookId])
      }
      onChanged?.()
    } catch (err) {
      setWantsPDF(!sendPDF)
      setError(err instanceof Error ? err.message : 'Failed to update sync format.')
    } finally {
      setToggling(false)
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <input
          id="kobo-sync-toggle"
          type="checkbox"
          checked={enabledState}
          disabled={(!canEnable && !enabledState) || toggling}
          onChange={handleToggle}
          className="rounded accent-[rgb(var(--color-accent))]"
          data-testid="kobo-sync-checkbox"
        />
        <label htmlFor="kobo-sync-toggle" className="text-sm text-subtle cursor-pointer">
          Kobo sync
        </label>
      </div>

      {!canEnable && !enabledState && (
        <p className="text-xs text-muted">Upload an EPUB or PDF to enable Kobo sync.</p>
      )}

      {enabledState && hasPdf && (
        <div className="space-y-1">
          <p className="text-xs text-muted">Send to Kobo as:</p>
          <div className="flex gap-3">
            <label className="flex items-center gap-1.5 text-xs text-subtle cursor-pointer">
              <input
                type="radio"
                name={`kobo-format-${bookId}`}
                checked={!wantsPDF}
                disabled={toggling}
                onChange={() => wantsPDF && handleFormatChange(false)}
                className="accent-[rgb(var(--color-accent))]"
                data-testid="kobo-format-kepub"
              />
              EPUB (converted)
            </label>
            <label className="flex items-center gap-1.5 text-xs text-subtle cursor-pointer">
              <input
                type="radio"
                name={`kobo-format-${bookId}`}
                checked={wantsPDF}
                disabled={toggling}
                onChange={() => !wantsPDF && handleFormatChange(true)}
                className="accent-[rgb(var(--color-accent))]"
                data-testid="kobo-format-pdf"
              />
              PDF (as-is)
            </label>
          </div>
        </div>
      )}

      {enabledState && !wantsPDF && kepubStatus && (
        <p
          className={`text-xs ${kepubStatus === 'failed' ? 'text-danger' : 'text-muted'}`}
          data-testid="kepub-status"
        >
          {kepubStatusLabel(kepubStatus)}
        </p>
      )}

      {error && <p className="text-xs text-danger">{error}</p>}
    </div>
  )
}
