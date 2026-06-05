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
import { Input } from '@/components/ui/input'

interface ShareModalProps {
  sharedWith: string[]
  onShare: (userId: string) => Promise<void>
  onUnshare: (userId: string) => Promise<void>
  onClose: () => void
}

export default function ShareModal({ sharedWith, onShare, onUnshare, onClose }: ShareModalProps) {
  const [shareInput, setShareInput] = useState('')
  const [shareError, setShareError] = useState<string | null>(null)

  const handleShare = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!shareInput.trim()) return
    setShareError(null)
    try {
      await onShare(shareInput.trim())
      setShareInput('')
    } catch (err) {
      setShareError(err instanceof Error ? err.message : 'Failed to share.')
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Sharing</DialogTitle>
          <DialogClose aria-label="Close">×</DialogClose>
        </DialogHeader>

        <form onSubmit={handleShare} className="flex gap-2 mb-3">
          <Input
            type="text"
            value={shareInput}
            onChange={(e) => setShareInput(e.target.value)}
            placeholder="User ID to share with"
            className="flex-1"
          />
          <Button type="submit" size="md">
            Share
          </Button>
        </form>

        {shareError && <p className="mb-3 text-sm text-danger">{shareError}</p>}

        {sharedWith.length > 0 ? (
          <ul className="space-y-2">
            {sharedWith.map((userId) => (
              <li key={userId} className="flex items-center justify-between text-sm">
                <span>{userId}</span>
                <Button
                  variant="link"
                  size="sm"
                  onClick={() => onUnshare(userId)}
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
