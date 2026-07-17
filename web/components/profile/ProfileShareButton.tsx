'use client'

import Link from 'next/link'
import { useState } from 'react'
import { useCurrentUser } from '@/hooks/useAuth'
import {
  useProfileShare,
  useCreateProfileShare,
  useDeleteProfileShare,
  type ProfileAppKey
} from '@/hooks/useProfile'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

function shareUrl(app: ProfileAppKey, token: string) {
  return `${window.location.origin}/profile/${app}/${token}`
}

// ProfileShareButton opens a public, read-only profile link for one app
// (books or games) — its own token, independent of the other app's. Sharing
// requires a display name (set in Settings) so visitors know whose profile
// they're viewing.
export default function ProfileShareButton({ app }: { app: ProfileAppKey }) {
  const { data: user } = useCurrentUser()
  const { data, mutate } = useProfileShare(app)
  const createShare = useCreateProfileShare(app)
  const deleteShare = useDeleteProfileShare(app)
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
    await navigator.clipboard.writeText(shareUrl(app, token))
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <>
      <Button variant="secondary" onClick={() => setOpen(true)}>
        Share profile
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Share your profile</DialogTitle>
            <DialogClose aria-label="Close">×</DialogClose>
          </DialogHeader>

          {!hasDisplayName ? (
            <div className="space-y-3">
              <p className="text-sm text-muted">
                Set a display name in settings before sharing your profile — visitors need to know
                whose profile they&apos;re viewing.
              </p>
              <Button asChild variant="secondary" size="sm">
                <Link href="/settings">Go to settings</Link>
              </Button>
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-muted">
                Share a read-only view of your {app}. Anyone with the link can see it — no account
                needed.
              </p>
              {token ? (
                <>
                  <div className="flex flex-col gap-2 sm:flex-row">
                    <Input readOnly value={shareUrl(app, token)} aria-label="Public profile link" />
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
