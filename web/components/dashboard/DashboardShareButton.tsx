'use client'

import Link from 'next/link'
import { useState } from 'react'
import { useCurrentUser } from '@/hooks/useAuth'
import {
  useDashboardShare,
  useCreateDashboardShare,
  useDeleteDashboardShare,
  type DashboardShareKind
} from '@/hooks/useDashboardShare'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

function shareUrl(kind: DashboardShareKind, token: string) {
  return `${window.location.origin}/dashboard/${kind}/${token}`
}

// DashboardShareButton opens a public, read-only dashboard link for one
// dashboard (games or reading) — its own token, independent of the other
// dashboard's. Sharing requires a display name (set in Settings) so
// visitors know whose dashboard they're viewing.
export default function DashboardShareButton({ kind }: { kind: DashboardShareKind }) {
  const { data: user } = useCurrentUser()
  const { data, mutate } = useDashboardShare(kind)
  const createShare = useCreateDashboardShare(kind)
  const deleteShare = useDeleteDashboardShare(kind)
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)

  const token = data?.share?.token
  const hasDisplayName = !!user?.displayName

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
    await navigator.clipboard.writeText(shareUrl(kind, token))
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <>
      <Button variant="secondary" onClick={() => setOpen(true)}>
        Share dashboard
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Share your dashboard</DialogTitle>
            <DialogClose aria-label="Close">×</DialogClose>
          </DialogHeader>

          {!hasDisplayName ? (
            <div className="space-y-3">
              <p className="text-sm text-muted">
                Set a display name in settings before sharing your dashboard — visitors need to know
                whose dashboard they&apos;re viewing.
              </p>
              <Button asChild variant="secondary" size="sm">
                <Link href="/settings">Go to settings</Link>
              </Button>
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-muted">
                Share a read-only view of your {kind} dashboard. Anyone with the link can see it —
                no account needed.
              </p>
              {token ? (
                <>
                  <div className="flex flex-col gap-2 sm:flex-row">
                    <Input
                      readOnly
                      value={shareUrl(kind, token)}
                      aria-label="Public dashboard link"
                    />
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
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
