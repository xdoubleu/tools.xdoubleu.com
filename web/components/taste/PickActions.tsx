'use client'

import { useState } from 'react'
import type { Recommendation } from '@/lib/gen/taste/v1/taste_pb'
import { usePickActions } from '@/hooks/useTaste'
import { FEEDBACK } from '@/lib/taste/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { ToggleIconButton } from '@/components/ui/toggle-icon-button'

/** Feedback, save and add-to-backlog controls for one pick. */
export default function PickActions({ pick }: { pick: Recommendation }) {
  const actions = usePickActions()
  // One action at a time, so the error shown is that action's.
  const [busy, setBusy] = useState(false)
  const [accepting, setAccepting] = useState(false)
  const [failed, setFailed] = useState(false)

  const run = async (call: () => Promise<void>, isAccept = false) => {
    if (busy) return
    setBusy(true)
    setAccepting(isAccept)
    setFailed(false)
    try {
      await call()
    } catch {
      setFailed(true)
    } finally {
      setBusy(false)
      setAccepting(false)
    }
  }

  const feedbackId = `taste-feedback-${pick.id}`
  return (
    <div className="mt-2 space-y-1">
      <div className="flex flex-wrap items-center gap-2">
        <div className="flex w-full items-center gap-2 sm:w-auto">
          <Label htmlFor={feedbackId} className="sr-only">
            How did {pick.title} land?
          </Label>
          <Select
            id={feedbackId}
            value={pick.feedback}
            disabled={busy}
            onChange={(e) => void run(() => actions.setFeedback(pick.id, e.target.value))}
            className="min-w-0 flex-1 sm:w-40 sm:flex-none"
          >
            <option value="">Rate…</option>
            {FEEDBACK.map((f) => (
              <option key={f.value} value={f.value}>
                {f.label}
              </option>
            ))}
          </Select>
          <ToggleIconButton
            active={pick.saved}
            onToggle={() => void run(() => actions.setSaved(pick.id, !pick.saved))}
            label={`Save ${pick.title}`}
            activeLabel={`Unsave ${pick.title}`}
            className="h-11 w-11 shrink-0"
          >
            <svg viewBox="0 0 16 16" className="h-4 w-4" fill="currentColor" aria-hidden="true">
              <path d="M3 1.5A1.5 1.5 0 0 1 4.5 0h7A1.5 1.5 0 0 1 13 1.5V16l-5-3-5 3V1.5Z" />
            </svg>
          </ToggleIconButton>
        </div>
        {pick.accepted ? (
          <Badge variant="success">In your backlog</Badge>
        ) : (
          pick.canAccept && (
            <Button
              variant="secondary"
              size="sm"
              className="min-h-11"
              disabled={busy}
              onClick={() => void run(() => actions.accept(pick.id), true)}
            >
              {accepting ? 'Adding…' : 'Add to backlog'}
            </Button>
          )
        )}
      </div>
      {failed && (
        <p className="text-xs text-danger" role="alert">
          That didn&apos;t save. Try again.
        </p>
      )}
    </div>
  )
}
