'use client'

import { useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { useContacts } from '@/hooks/useContacts'

export interface ShareEntry {
  userId: string
  displayName: string
  canEdit: boolean
}

interface ShareModalProps {
  title?: string
  shares: ShareEntry[]
  onShare: (contactUserId: string, canEdit: boolean) => Promise<void>
  onUnshare: (userId: string) => Promise<void>
  onClose: () => void
}

export default function ShareModal({
  title = 'Sharing',
  shares,
  onShare,
  onUnshare,
  onClose
}: ShareModalProps) {
  const { data: contactsData } = useContacts()
  const [selectedContact, setSelectedContact] = useState('')
  const [canEdit, setCanEdit] = useState(true)
  const [shareError, setShareError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const sharedIds = new Set(shares.map((s) => s.userId))
  const available = (contactsData?.contacts ?? []).filter((c) => !sharedIds.has(c.contactUserId))

  const handleShare = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedContact) return
    setShareError(null)
    setBusy(true)
    try {
      await onShare(selectedContact, canEdit)
      setSelectedContact('')
    } catch (err) {
      setShareError(err instanceof Error ? err.message : 'Failed to share.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogClose aria-label="Close">×</DialogClose>
        </DialogHeader>

        {available.length > 0 ? (
          <form onSubmit={handleShare} className="mb-3 flex flex-wrap items-center gap-2">
            <Select
              aria-label="Contact to share with"
              value={selectedContact}
              onChange={(e) => setSelectedContact(e.target.value)}
              className="min-w-40 flex-1"
            >
              <option value="">-- Select a contact --</option>
              {available.map((c) => (
                <option key={c.id} value={c.contactUserId}>
                  {c.displayName}
                </option>
              ))}
            </Select>
            <Select
              aria-label="Permission"
              value={canEdit ? 'edit' : 'view'}
              onChange={(e) => setCanEdit(e.target.value === 'edit')}
              className="w-auto"
            >
              <option value="edit">Can edit</option>
              <option value="view">View only</option>
            </Select>
            <Button type="submit" size="md" disabled={busy || !selectedContact}>
              Share
            </Button>
          </form>
        ) : (
          <p className="mb-3 text-sm text-muted">
            {contactsData && contactsData.contacts.length === 0
              ? 'Add contacts first to share with them.'
              : 'Shared with all your contacts.'}
          </p>
        )}

        {shareError && <p className="mb-3 text-sm text-danger">{shareError}</p>}

        {shares.length > 0 ? (
          <ul className="space-y-2">
            {shares.map((s) => (
              <li key={s.userId} className="flex items-center justify-between gap-2 text-sm">
                <span className="flex items-center gap-2">
                  {s.displayName}
                  <Badge variant={s.canEdit ? 'success' : 'secondary'}>
                    {s.canEdit ? 'Can edit' : 'View only'}
                  </Badge>
                </span>
                <Button
                  variant="link"
                  size="sm"
                  onClick={() => onUnshare(s.userId)}
                  className="h-auto px-0 text-xs text-danger focus-visible:ring-danger/50"
                >
                  Unshare
                </Button>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-muted">Not shared with anyone yet.</p>
        )}
      </DialogContent>
    </Dialog>
  )
}
