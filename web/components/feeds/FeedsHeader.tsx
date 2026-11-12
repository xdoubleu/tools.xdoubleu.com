import { useState } from 'react'
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
import { PageHeader } from '@/components/ui/page-header'
import { Popover } from '@/components/ui/popover'

function MoreMenu({ isAdmin }: { isAdmin: boolean }) {
  const links = [
    { href: '/feeds/stats', label: 'Stats' },
    { href: '/feeds/settings', label: 'Settings' },
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

// Feed management opens as a sheet from the header; the empty inbox points to it.
export default function FeedsHeader() {
  const { data: currentUser } = useCurrentUser()
  const isAdmin = currentUser?.role === 'admin'
  const [open, setOpen] = useState(false)

  return (
    <>
      <PageHeader
        title="Feeds"
        actions={
          <>
            <Button variant="secondary" aria-haspopup="dialog" onClick={() => setOpen(true)}>
              Manage feeds
            </Button>
            <MoreMenu isAdmin={isAdmin} />
          </>
        }
      />

      <Dialog open={open} onOpenChange={setOpen}>
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
