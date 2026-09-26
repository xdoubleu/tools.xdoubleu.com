'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useEnableKoboSync, useToggleTag, useKEPUBStatus } from '@/hooks/useBooks'
import { swrKeys } from '@/lib/swrKeys'
import { Checkbox } from '@/components/ui/checkbox'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { cn } from '@/lib/cn'

interface KoboSyncToggleProps {
  bookId: string
  enabled: boolean
  tags: string[]
  onChanged?: () => void
}

function kepubStatusLabel(status: string): string {
  if (status === 'converting') return 'Preparing for Kobo…'
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
        mutate(swrKeys.kepubStatus(bookId))
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
    // Optimistic flip.
    setWantsPDF(sendPDF)
    setToggling(true)
    setError(null)
    try {
      if (sendPDF) {
        // The tag serves raw PDF.
        await toggleTag(bookId, 'kobo-format-pdf')
      } else {
        // Re-trigger conversion so the KEPUB is ready.
        await toggleTag(bookId, 'kobo-format-pdf')
        await enableKoboSync(bookId)
        mutate(swrKeys.kepubStatus(bookId))
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
      <Checkbox
        id="kobo-sync-toggle"
        checked={enabledState}
        disabled={(!canEnable && !enabledState) || toggling}
        onChange={handleToggle}
        data-testid="kobo-sync-checkbox"
        label={<span className="text-sm text-subtle">Kobo sync</span>}
      />

      {!canEnable && !enabledState && (
        <p className="text-xs text-muted">Upload an EPUB or PDF to enable Kobo sync.</p>
      )}

      {enabledState && hasPdf && (
        <div className="space-y-1">
          <p className="text-xs text-muted">Send to Kobo as:</p>
          <RadioGroup
            name={`kobo-format-${bookId}`}
            value={wantsPDF ? 'pdf' : 'kepub'}
            onChange={(v) => handleFormatChange(v === 'pdf')}
            aria-label="Send to Kobo as"
            className="flex-row flex-wrap gap-x-4 gap-y-0"
          >
            <RadioGroupItem
              value="kepub"
              label="EPUB (converted)"
              disabled={toggling}
              data-testid="kobo-format-kepub"
            />
            <RadioGroupItem
              value="pdf"
              label="PDF (as-is)"
              disabled={toggling}
              data-testid="kobo-format-pdf"
            />
          </RadioGroup>
        </div>
      )}

      {enabledState && !wantsPDF && kepubStatus && (
        <p
          className={cn('text-xs', kepubStatus === 'failed' ? 'text-danger' : 'text-muted')}
          data-testid="kepub-status"
        >
          {kepubStatusLabel(kepubStatus)}
        </p>
      )}

      {error && <p className="text-xs text-danger">{error}</p>}
    </div>
  )
}
