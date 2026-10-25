import { Link } from '@/components/ui/link'
import { useState } from 'react'
import { useAsyncAction } from '@/hooks/useAsyncAction'
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
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

function shareUrl(kind: DashboardShareKind, token: string) {
  return `${window.location.origin}/dashboard/${kind}/${token}`
}

// Opens one dashboard's public read-only link (own token). Requires a
// display name so visitors know whose it is.
export default function DashboardShareButton({ kind }: { kind: DashboardShareKind }) {
  const { data: user } = useCurrentUser()
  const { data, mutate } = useDashboardShare(kind)
  const createShare = useCreateDashboardShare(kind)
  const deleteShare = useDeleteDashboardShare(kind)
  const [open, setOpen] = useState(false)
  const { run, pending: busy, error } = useAsyncAction()
  const [copied, setCopied] = useState(false)

  const token = data?.share?.token
  const hasDisplayName = !!user?.displayName

  const save = (action: () => Promise<unknown>, failure: string) =>
    run(async () => {
      await action()
      await mutate()
    }, failure)

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
              {error && <Alert tone="danger">{error}</Alert>}
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
                      onClick={() => save(createShare, "Couldn't regenerate the link. Try again.")}
                    >
                      Regenerate link
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      disabled={busy}
                      onClick={() => save(deleteShare, "Couldn't disable sharing. Try again.")}
                    >
                      Disable sharing
                    </Button>
                  </div>
                  <p className="text-xs text-muted">
                    Regenerating or disabling immediately stops the current link from working.
                  </p>
                </>
              ) : (
                <Button
                  disabled={busy}
                  onClick={() => save(createShare, "Couldn't create the share link. Try again.")}
                >
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
