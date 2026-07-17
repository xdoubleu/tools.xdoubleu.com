'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { ConnectError, Code } from '@connectrpc/connect'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { useAddBookByURL } from '@/hooks/useBooks'
import { swrKeys } from '@/lib/swrKeys'

function errorMessage(err: unknown): string {
  if (err instanceof ConnectError) {
    switch (err.code) {
      case Code.InvalidArgument:
        return 'That page has no readable article content (or the URL is invalid).'
      case Code.NotFound:
        return 'No arXiv paper found for that id.'
      case Code.Unavailable:
        return 'The page could not be fetched — it may be down or paywalled.'
      case Code.ResourceExhausted:
        return 'The file behind that URL is too large.'
      default:
        return 'Adding the URL failed. Please try again.'
    }
  }
  return 'Adding the URL failed. Please try again.'
}

// AddByUrlDialog ingests a pasted URL as a library item: arXiv links become
// papers (metadata + PDF), other pages become readability-extracted article
// EPUBs. Pasting something already in the library is not an error — it just
// reports so.
export default function AddByUrlDialog({
  open,
  onOpenChange,
  onAdded
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdded?: () => void
}) {
  const addByURL = useAddBookByURL()
  const [url, setUrl] = useState('')
  const [category, setCategory] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const reset = () => {
    setUrl('')
    setCategory('')
    setError('')
    setNotice('')
  }

  const submit = async () => {
    if (!url.trim() || busy) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      const resp = await addByURL(url.trim(), category)
      await mutate(swrKeys.books)
      onAdded?.()
      if (resp.alreadyInLibrary) {
        setNotice('Already in your library.')
      } else {
        reset()
        onOpenChange(false)
      }
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) reset()
        onOpenChange(next)
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add by URL</DialogTitle>
        </DialogHeader>
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault()
            void submit()
          }}
        >
          <Input
            type="url"
            required
            placeholder="https://arxiv.org/abs/… or an article URL"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            aria-label="URL"
          />
          <Select
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            aria-label="Category"
          >
            <option value="">Auto-detect category</option>
            <option value="paper">Paper</option>
            <option value="article">Article</option>
          </Select>
          {error && <p className="text-sm text-danger">{error}</p>}
          {notice && <p className="text-sm text-muted">{notice}</p>}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy || !url.trim()}>
              {busy ? 'Adding…' : 'Add'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
