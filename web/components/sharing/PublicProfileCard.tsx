'use client'

import { useState } from 'react'
import { useProfileShare, useCreateProfileShare, useDeleteProfileShare } from '@/hooks/useProfile'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

function shareUrl(token: string) {
  return `${window.location.origin}/profile/${token}`
}

// PublicProfileCard manages the public read-only profile link: anyone with
// the link can browse the owner's books and games dashboards without an
// account. Regenerating invalidates the previous link.
export default function PublicProfileCard() {
  const { data, mutate } = useProfileShare()
  const createShare = useCreateProfileShare()
  const deleteShare = useDeleteProfileShare()
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)

  const token = data?.share?.token

  const run = async (action: () => Promise<unknown>) => {
    setBusy(true)
    try {
      await action()
      await mutate()
    } finally {
      setBusy(false)
    }
  }

  const copyLink = async () => {
    if (!token) return
    await navigator.clipboard.writeText(shareUrl(token))
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Public profile</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-sm text-muted">
          Share a read-only view of your books and games dashboards. Anyone with the link can see
          your libraries and backlogs — no account needed.
        </p>
        {token ? (
          <>
            <div className="flex flex-col gap-2 sm:flex-row">
              <Input readOnly value={shareUrl(token)} aria-label="Public profile link" />
              <Button variant="secondary" onClick={copyLink} disabled={busy}>
                {copied ? 'Copied!' : 'Copy link'}
              </Button>
            </div>
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                disabled={busy}
                onClick={() => run(createShare)}
              >
                Regenerate link
              </Button>
              <Button
                variant="destructive"
                size="sm"
                disabled={busy}
                onClick={() => run(deleteShare)}
              >
                Disable sharing
              </Button>
            </div>
            <p className="text-xs text-muted">
              Regenerating or disabling immediately stops the current link from working.
            </p>
          </>
        ) : (
          <Button disabled={busy} onClick={() => run(createShare)}>
            Create share link
          </Button>
        )}
      </CardContent>
    </Card>
  )
}
