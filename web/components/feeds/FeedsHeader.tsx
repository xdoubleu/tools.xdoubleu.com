import { useEffect, useState } from 'react'
import { useFeeds } from '@/hooks/useFeeds'
import { useCurrentUser } from '@/hooks/useAuth'
import FeedManager from '@/components/feeds/FeedManager'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Link } from '@/components/ui/link'
import { PageHeader, PageHeaderLink, PageHeaderSettingsLink } from '@/components/ui/page-header'
import { Popover } from '@/components/ui/popover'

function MoreMenu({ isAdmin }: { isAdmin: boolean }) {
  const links = [
    { href: '/feeds/expired', label: 'Expired' },
    { href: '/feeds/filtered', label: 'Filtered' },
    ...(isAdmin ? [{ href: '/feeds/health', label: 'Health' }] : [])
  ]
  return (
    <Popover
      aria-label="More feed pages"
      trigger={({ open, onClick }) => (
        <Button variant="secondary" aria-haspopup="menu" aria-expanded={open} onClick={onClick}>
          More
        </Button>
      )}
    >
      <div role="menu" className="flex min-w-36 flex-col">
        {links.map((l) => (
          <Button key={l.href} asChild variant="ghost" className="justify-start">
            <Link href={l.href} role="menuitem">
              {l.label}
            </Link>
          </Button>
        ))}
      </div>
    </Popover>
  )
}

// Feed management opens as a sheet; it opens itself while there are no feeds.
// useFeeds() is deduped by SWR.
export default function FeedsHeader() {
  const { data } = useFeeds()
  const { data: currentUser } = useCurrentUser()
  const isAdmin = currentUser?.role === 'admin'
  const [open, setOpen] = useState(false)
  const [touched, setTouched] = useState(false)

  useEffect(() => {
    if (!touched && data && data.feeds.length === 0) setOpen(true)
  }, [touched, data])

  const change = (next: boolean) => {
    setTouched(true)
    setOpen(next)
  }

  return (
    <>
      <PageHeader
        title="Feeds"
        actions={
          <>
            <Button variant="secondary" aria-haspopup="dialog" onClick={() => change(true)}>
              Manage feeds
            </Button>
            <PageHeaderLink href="/feeds/stats">Stats</PageHeaderLink>
            <PageHeaderSettingsLink href="/feeds/settings" />
            <MoreMenu isAdmin={isAdmin} />
          </>
        }
      />

      <Dialog open={open} onOpenChange={change}>
        <DialogContent side="sheet" className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Manage feeds</DialogTitle>
            <DialogClose />
          </DialogHeader>
          <DialogDescription className="mb-3">
            Subscribe to blogs, news feeds, and email newsletters. New posts land here as items to
            read.
          </DialogDescription>
          <FeedManager />
        </DialogContent>
      </Dialog>
    </>
  )
}
